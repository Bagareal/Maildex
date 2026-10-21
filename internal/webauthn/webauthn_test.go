package webauthn

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"testing"
)

// Minimal CBOR encoding helpers for building fake authenticator output.
func cHead(major byte, n int) []byte {
	if n < 24 {
		return []byte{major<<5 | byte(n)}
	}
	if n < 256 {
		return []byte{major<<5 | 24, byte(n)}
	}
	return []byte{major<<5 | 25, byte(n >> 8), byte(n)}
}
func cBytes(b []byte) []byte { return append(cHead(2, len(b)), b...) }
func cText(s string) []byte  { return append(cHead(3, len(s)), s...) }
func cInt(n int) []byte {
	if n >= 0 {
		return cHead(0, n)
	}
	return cHead(1, -1-n)
}

type fakeAuthenticator struct {
	key    *ecdsa.PrivateKey
	credID []byte
}

func (f *fakeAuthenticator) cose() []byte {
	x := f.key.X.FillBytes(make([]byte, 32))
	y := f.key.Y.FillBytes(make([]byte, 32))
	out := cHead(5, 5)
	out = append(out, cInt(1)...)
	out = append(out, cInt(2)...)
	out = append(out, cInt(3)...)
	out = append(out, cInt(AlgES256)...)
	out = append(out, cInt(-1)...)
	out = append(out, cInt(1)...)
	out = append(out, cInt(-2)...)
	out = append(out, cBytes(x)...)
	out = append(out, cInt(-3)...)
	out = append(out, cBytes(y)...)
	return out
}

func authDataFor(rpID string, flags byte, counter uint32, attested []byte) []byte {
	h := sha256.Sum256([]byte(rpID))
	b := append([]byte{}, h[:]...)
	b = append(b, flags)
	b = binary.BigEndian.AppendUint32(b, counter)
	return append(b, attested...)
}

func clientDataFor(typ string, exp Expect) []byte {
	b, _ := json.Marshal(map[string]string{"type": typ, "challenge": exp.Challenge, "origin": exp.Origin})
	return b
}

func newAuthenticator(t *testing.T) *fakeAuthenticator {
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return &fakeAuthenticator{key: k, credID: []byte("credential-id-123")}
}

func (f *fakeAuthenticator) register(exp Expect) (attObj, cd []byte) {
	att := append(make([]byte, 16), byte(len(f.credID)>>8), byte(len(f.credID)))
	att = append(att, f.credID...)
	att = append(att, f.cose()...)
	ad := authDataFor(exp.RPID, flagUP|flagUV|flagAT, 0, att)
	obj := cHead(5, 3)
	obj = append(obj, cText("fmt")...)
	obj = append(obj, cText("none")...)
	obj = append(obj, cText("attStmt")...)
	obj = append(obj, cHead(5, 0)...)
	obj = append(obj, cText("authData")...)
	obj = append(obj, cBytes(ad)...)
	return obj, clientDataFor("webauthn.create", exp)
}

func (f *fakeAuthenticator) assert(exp Expect, flags byte, counter uint32) (ad, cd, sig []byte) {
	ad = authDataFor(exp.RPID, flags, counter, nil)
	cd = clientDataFor("webauthn.get", exp)
	cdh := sha256.Sum256(cd)
	digest := sha256.Sum256(append(append([]byte{}, ad...), cdh[:]...))
	sig, err := ecdsa.SignASN1(rand.Reader, f.key, digest[:])
	if err != nil {
		panic(err)
	}
	return
}

var exp = Expect{Challenge: "Y2hhbGxlbmdl", Origin: "https://maildex.lan", RPID: "maildex.lan"}

func TestRegisterAndAssert(t *testing.T) {
	f := newAuthenticator(t)
	obj, cd := f.register(exp)
	reg, err := VerifyRegistration(obj, cd, exp)
	if err != nil {
		t.Fatalf("registration: %v", err)
	}
	if string(reg.CredentialID) != string(f.credID) {
		t.Fatal("credential id mismatch")
	}

	ad, acd, sig := f.assert(exp, flagUP|flagUV, 7)
	n, err := VerifyAssertion(reg.PublicKey, ad, acd, sig, exp)
	if err != nil || n != 7 {
		t.Fatalf("assertion: n=%d err=%v", n, err)
	}
}

func TestAssertionRejections(t *testing.T) {
	f := newAuthenticator(t)
	obj, cd := f.register(exp)
	reg, err := VerifyRegistration(obj, cd, exp)
	if err != nil {
		t.Fatal(err)
	}
	ad, acd, sig := f.assert(exp, flagUP|flagUV, 1)

	cases := map[string]func() error{
		"wrong challenge": func() error {
			e := exp
			e.Challenge = "other"
			_, err := VerifyAssertion(reg.PublicKey, ad, acd, sig, e)
			return err
		},
		"wrong origin": func() error {
			e := exp
			e.Origin = "https://evil.example"
			_, err := VerifyAssertion(reg.PublicKey, ad, acd, sig, e)
			return err
		},
		"wrong rp id": func() error {
			e := exp
			e.RPID = "evil.example"
			_, err := VerifyAssertion(reg.PublicKey, ad, acd, sig, e)
			return err
		},
		"tampered signature": func() error {
			bad := append([]byte{}, sig...)
			bad[len(bad)-1] ^= 0xff
			_, err := VerifyAssertion(reg.PublicKey, ad, acd, bad, exp)
			return err
		},
		"no user verification": func() error {
			ad2, acd2, sig2 := f.assert(exp, flagUP, 1)
			_, err := VerifyAssertion(reg.PublicKey, ad2, acd2, sig2, exp)
			return err
		},
		"other key": func() error {
			g := newAuthenticator(t)
			ad2, acd2, sig2 := g.assert(exp, flagUP|flagUV, 1)
			_, err := VerifyAssertion(reg.PublicKey, ad2, acd2, sig2, exp)
			return err
		},
	}
	for name, fn := range cases {
		if fn() == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestRegistrationRejectsGarbage(t *testing.T) {
	if _, err := VerifyRegistration([]byte{0xff, 0x00}, clientDataFor("webauthn.create", exp), exp); err == nil {
		t.Fatal("expected error")
	}
}
