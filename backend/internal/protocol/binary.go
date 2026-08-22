package protocol

import (
	"encoding/binary"
	"math"
)

type CursorSample struct {
	UserIdx uint32
	X       float32
	Y       float32
}

func EncodeCursorC2S(x, y float32) []byte {
	buf := make([]byte, 9)
	buf[0] = BinCursorC2S
	binary.LittleEndian.PutUint32(buf[1:5], math.Float32bits(x))
	binary.LittleEndian.PutUint32(buf[5:9], math.Float32bits(y))
	return buf
}

func DecodeCursorC2S(b []byte) (x, y float32, ok bool) {
	if len(b) < 9 || b[0] != BinCursorC2S {
		return 0, 0, false
	}
	x = math.Float32frombits(binary.LittleEndian.Uint32(b[1:5]))
	y = math.Float32frombits(binary.LittleEndian.Uint32(b[5:9]))
	if !finite32(x) || !finite32(y) {
		return 0, 0, false
	}
	return x, y, true
}

func EncodeCursorS2C(samples []CursorSample) []byte {
	n := len(samples)
	if n > 255 {
		n = 255
		samples = samples[:255]
	}
	buf := make([]byte, 2+n*12)
	buf[0] = BinCursorS2C
	buf[1] = byte(n)
	off := 2
	for i := 0; i < n; i++ {
		s := samples[i]
		binary.LittleEndian.PutUint32(buf[off:off+4], s.UserIdx)
		binary.LittleEndian.PutUint32(buf[off+4:off+8], math.Float32bits(s.X))
		binary.LittleEndian.PutUint32(buf[off+8:off+12], math.Float32bits(s.Y))
		off += 12
	}
	return buf
}

func DecodeCursorS2C(b []byte) ([]CursorSample, bool) {
	if len(b) < 2 || b[0] != BinCursorS2C {
		return nil, false
	}
	n := int(b[1])
	need := 2 + n*12
	if len(b) < need {
		return nil, false
	}
	out := make([]CursorSample, n)
	off := 2
	for i := 0; i < n; i++ {
		out[i].UserIdx = binary.LittleEndian.Uint32(b[off : off+4])
		out[i].X = math.Float32frombits(binary.LittleEndian.Uint32(b[off+4 : off+8]))
		out[i].Y = math.Float32frombits(binary.LittleEndian.Uint32(b[off+8 : off+12]))
		if !finite32(out[i].X) || !finite32(out[i].Y) {
			return nil, false
		}
		off += 12
	}
	return out, true
}

func finite32(v float32) bool {
	return !(math.IsNaN(float64(v)) || math.IsInf(float64(v), 0))
}
