package webauthn

import (
	"crypto/elliptic"
	"encoding/binary"
	"errors"
)

func ellipticP256() elliptic.Curve { return elliptic.P256() }

// cborDecode decodes one CBOR item and returns it with the number of bytes
// consumed. It handles what WebAuthn uses: integers, byte/text strings, arrays,
// maps and simple values. Maps decode to map[any]any with int64 or string keys.
func cborDecode(b []byte) (any, int, error) {
	return cborItem(b, 0)
}

const maxDepth = 8

func cborItem(b []byte, depth int) (any, int, error) {
	if depth > maxDepth {
		return nil, 0, errors.New("cbor: nesting too deep")
	}
	if len(b) == 0 {
		return nil, 0, errors.New("cbor: unexpected end")
	}
	major, info := b[0]>>5, b[0]&0x1f
	pos := 1
	var arg uint64
	switch {
	case info < 24:
		arg = uint64(info)
	case info == 24:
		if len(b) < 2 {
			return nil, 0, errors.New("cbor: truncated")
		}
		arg, pos = uint64(b[1]), 2
	case info == 25:
		if len(b) < 3 {
			return nil, 0, errors.New("cbor: truncated")
		}
		arg, pos = uint64(binary.BigEndian.Uint16(b[1:])), 3
	case info == 26:
		if len(b) < 5 {
			return nil, 0, errors.New("cbor: truncated")
		}
		arg, pos = uint64(binary.BigEndian.Uint32(b[1:])), 5
	case info == 27:
		if len(b) < 9 {
			return nil, 0, errors.New("cbor: truncated")
		}
		arg, pos = binary.BigEndian.Uint64(b[1:]), 9
	default:
		return nil, 0, errors.New("cbor: indefinite lengths not supported")
	}
	switch major {
	case 0:
		return int64(arg), pos, nil
	case 1:
		return -1 - int64(arg), pos, nil
	case 2, 3:
		if arg > uint64(len(b)-pos) {
			return nil, 0, errors.New("cbor: string truncated")
		}
		raw := b[pos : pos+int(arg)]
		if major == 3 {
			return string(raw), pos + int(arg), nil
		}
		return append([]byte{}, raw...), pos + int(arg), nil
	case 4:
		if arg > uint64(len(b)) {
			return nil, 0, errors.New("cbor: array too long")
		}
		out := make([]any, 0, arg)
		for i := uint64(0); i < arg; i++ {
			v, n, err := cborItem(b[pos:], depth+1)
			if err != nil {
				return nil, 0, err
			}
			out = append(out, v)
			pos += n
		}
		return out, pos, nil
	case 5:
		if arg > uint64(len(b)) {
			return nil, 0, errors.New("cbor: map too long")
		}
		out := make(map[any]any, arg)
		for i := uint64(0); i < arg; i++ {
			k, n, err := cborItem(b[pos:], depth+1)
			if err != nil {
				return nil, 0, err
			}
			pos += n
			switch k.(type) {
			case int64, string:
			default:
				return nil, 0, errors.New("cbor: unsupported map key")
			}
			v, n, err := cborItem(b[pos:], depth+1)
			if err != nil {
				return nil, 0, err
			}
			pos += n
			out[k] = v
		}
		return out, pos, nil
	case 7:
		switch info {
		case 20:
			return false, pos, nil
		case 21:
			return true, pos, nil
		case 22, 23:
			return nil, pos, nil
		}
	}
	return nil, 0, errors.New("cbor: unsupported item")
}
