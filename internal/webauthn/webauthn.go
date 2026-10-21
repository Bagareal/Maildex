// Package webauthn is a deliberately small WebAuthn relying-party
// implementation: just enough to enrol a platform authenticator (fingerprint,
// Face ID, Windows Hello) and verify its assertions. It supports "none"
// attestation only, which is what the browser is asked for; the credential is
// trusted on first use, as in any single-user self-hosted app.
package webauthn

import (
	"bytes"
	"crypto"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
)

const (
	AlgES256 = -7
	AlgRS256 = -257

	flagUP = 0x01
	flagUV = 0x04
	flagAT = 0x40
)

// Expect describes what a ceremony must match.
type Expect struct {
	Challenge string // base64url, as issued by the server
	Origin    string // e.g. https://maildex.lan
	RPID      string // e.g. maildex.lan
}

type Registered struct {
	CredentialID []byte
	PublicKey    []byte // raw COSE_Key
	SignCount    uint32
}

type clientData struct {
	Type      string `json:"type"`
	Challenge string `json:"challenge"`
	Origin    string `json:"origin"`
}

func checkClientData(raw []byte, wantType string, exp Expect) error {
	var cd clientData
	if err := json.Unmarshal(raw, &cd); err != nil {
		return fmt.Errorf("client data: %w", err)
	}
	switch {
	case cd.Type != wantType:
		return errors.New("wrong ceremony type")
	case cd.Challenge != exp.Challenge:
		return errors.New("challenge mismatch")
	case cd.Origin != exp.Origin:
		return fmt.Errorf("origin mismatch (%s)", cd.Origin)
	}
	return nil
}

type authData struct {
	rpIDHash  []byte
	flags     byte
	signCount uint32
	credID    []byte
	coseKey   []byte
}

func parseAuthData(b []byte) (*authData, error) {
	if len(b) < 37 {
		return nil, errors.New("authenticator data too short")
	}
	a := &authData{rpIDHash: b[:32], flags: b[32], signCount: binary.BigEndian.Uint32(b[33:37])}
	if a.flags&flagAT == 0 {
		return a, nil
	}
	rest := b[37:]
	if len(rest) < 18 {
		return nil, errors.New("attested credential data too short")
	}
	n := int(binary.BigEndian.Uint16(rest[16:18]))
	rest = rest[18:]
	if len(rest) < n {
		return nil, errors.New("credential id truncated")
	}
	a.credID = rest[:n]
	rest = rest[n:]
	_, used, err := cborDecode(rest)
	if err != nil {
		return nil, fmt.Errorf("public key: %w", err)
	}
	a.coseKey = rest[:used]
	return a, nil
}

func checkAuthData(a *authData, exp Expect) error {
	h := sha256.Sum256([]byte(exp.RPID))
	if !bytes.Equal(a.rpIDHash, h[:]) {
		return errors.New("rp id hash mismatch")
	}
	if a.flags&flagUP == 0 || a.flags&flagUV == 0 {
		return errors.New("user verification required")
	}
	return nil
}

// VerifyRegistration validates a navigator.credentials.create() response.
func VerifyRegistration(attestationObject, clientDataJSON []byte, exp Expect) (*Registered, error) {
	if err := checkClientData(clientDataJSON, "webauthn.create", exp); err != nil {
		return nil, err
	}
	v, _, err := cborDecode(attestationObject)
	if err != nil {
		return nil, fmt.Errorf("attestation object: %w", err)
	}
	m, ok := v.(map[any]any)
	if !ok {
		return nil, errors.New("attestation object is not a map")
	}
	raw, ok := m["authData"].([]byte)
	if !ok {
		return nil, errors.New("missing authData")
	}
	a, err := parseAuthData(raw)
	if err != nil {
		return nil, err
	}
	if err := checkAuthData(a, exp); err != nil {
		return nil, err
	}
	if a.credID == nil {
		return nil, errors.New("no credential in response")
	}
	if _, err := parsePublicKey(a.coseKey); err != nil {
		return nil, err
	}
	return &Registered{CredentialID: a.credID, PublicKey: a.coseKey, SignCount: a.signCount}, nil
}

// VerifyAssertion validates a navigator.credentials.get() response against a
// stored COSE public key and returns the new signature counter.
func VerifyAssertion(coseKey, authenticatorData, clientDataJSON, signature []byte, exp Expect) (uint32, error) {
	if err := checkClientData(clientDataJSON, "webauthn.get", exp); err != nil {
		return 0, err
	}
	a, err := parseAuthData(authenticatorData)
	if err != nil {
		return 0, err
	}
	if err := checkAuthData(a, exp); err != nil {
		return 0, err
	}
	pub, err := parsePublicKey(coseKey)
	if err != nil {
		return 0, err
	}
	cdHash := sha256.Sum256(clientDataJSON)
	signed := sha256.Sum256(append(append([]byte{}, authenticatorData...), cdHash[:]...))
	if err := pub.verify(signed[:], signature); err != nil {
		return 0, err
	}
	return a.signCount, nil
}

type publicKey struct {
	ec  *ecdsa.PublicKey
	rsa *rsa.PublicKey
}

func (p publicKey) verify(digest, sig []byte) error {
	if p.ec != nil {
		if !ecdsa.VerifyASN1(p.ec, digest, sig) {
			return errors.New("bad signature")
		}
		return nil
	}
	if err := rsa.VerifyPKCS1v15(p.rsa, crypto.SHA256, digest, sig); err != nil {
		return errors.New("bad signature")
	}
	return nil
}

func parsePublicKey(cose []byte) (publicKey, error) {
	v, _, err := cborDecode(cose)
	if err != nil {
		return publicKey{}, err
	}
	m, ok := v.(map[any]any)
	if !ok {
		return publicKey{}, errors.New("cose key is not a map")
	}
	alg, _ := m[int64(3)].(int64)
	switch alg {
	case AlgES256:
		x, _ := m[int64(-2)].([]byte)
		y, _ := m[int64(-3)].([]byte)
		if len(x) != 32 || len(y) != 32 {
			return publicKey{}, errors.New("bad EC key")
		}
		// NewPublicKey rejects points that are not on the curve.
		if _, err := ecdh.P256().NewPublicKey(append([]byte{4}, append(x, y...)...)); err != nil {
			return publicKey{}, errors.New("EC point not on curve")
		}
		return publicKey{ec: &ecdsa.PublicKey{Curve: ellipticP256(), X: new(big.Int).SetBytes(x), Y: new(big.Int).SetBytes(y)}}, nil
	case AlgRS256:
		n, _ := m[int64(-1)].([]byte)
		e, _ := m[int64(-2)].([]byte)
		if len(n) < 128 || len(e) == 0 || len(e) > 4 {
			return publicKey{}, errors.New("bad RSA key")
		}
		return publicKey{rsa: &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: int(new(big.Int).SetBytes(e).Int64())}}, nil
	}
	return publicKey{}, fmt.Errorf("unsupported algorithm %d", alg)
}

func B64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func FromB64(s string) ([]byte, error) { return base64.RawURLEncoding.DecodeString(s) }
