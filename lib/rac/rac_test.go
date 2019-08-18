// Copyright 2019 The Wuffs Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package rac

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"testing"
)

const bytesPerHexDumpLine = 79

const fakeCodec = Codec(0xEE)

var unhex = [256]uint8{
	'0': 0x00, '1': 0x01, '2': 0x02, '3': 0x03, '4': 0x04,
	'5': 0x05, '6': 0x06, '7': 0x07, '8': 0x08, '9': 0x09,
	'A': 0x0A, 'B': 0x0B, 'C': 0x0C, 'D': 0x0D, 'E': 0x0E, 'F': 0x0F,
	'a': 0x0A, 'b': 0x0B, 'c': 0x0C, 'd': 0x0D, 'e': 0x0E, 'f': 0x0F,
}

func undoHexDump(s string) (ret []byte) {
	for s != "" {
		for i := 0; i < 16; i++ {
			pos := 10 + 3*i
			if i > 7 {
				pos++
			}
			c0 := s[pos+0]
			c1 := s[pos+1]
			if c0 == ' ' {
				break
			}
			ret = append(ret, (unhex[c0]<<4)|unhex[c1])
		}
		if n := strings.IndexByte(s, '\n'); n >= 0 {
			s = s[n+1:]
		} else {
			break
		}
	}
	return ret
}

const writerWantEmpty = "" +
	"00000000  72 c3 63 01 0d f8 00 ff  00 00 00 00 00 00 00 00  |r.c.............|\n" +
	"00000010  20 00 00 00 00 00 01 ff  20 00 00 00 00 00 01 01  | ....... .......|\n"

const writerWantILAEnd = "" +
	"00000000  72 c3 63 00 52 72 72 53  73 41 61 61 42 62 62 62  |r.c.RrrSsAaaBbbb|\n" +
	"00000010  43 63 63 63 63 63 63 63  63 63 31 32 72 c3 63 05  |Cccccccccc12r.c.|\n" +
	"00000020  1d 45 00 ff 00 00 00 00  00 00 00 ff 00 00 00 00  |.E..............|\n" +
	"00000030  00 00 00 ff 11 00 00 00  00 00 00 ff 33 00 00 00  |............3...|\n" +
	"00000040  00 00 00 01 77 00 00 00  00 00 00 ee 04 00 00 00  |....w...........|\n" +
	"00000050  00 00 01 ff 07 00 00 00  00 00 01 ff 09 00 00 00  |................|\n" +
	"00000060  00 00 01 ff 0c 00 00 00  00 00 01 00 10 00 00 00  |................|\n" +
	"00000070  00 00 01 00 7c 00 00 00  00 00 01 05              |....|.......|\n"

const writerWantILAEndCPageSize8 = "" +
	"00000000  72 c3 63 00 52 72 72 00  53 73 41 61 61 00 00 00  |r.c.Rrr.SsAaa...|\n" +
	"00000010  42 62 62 62 43 63 63 63  63 63 63 63 63 63 31 32  |BbbbCccccccccc12|\n" +
	"00000020  72 c3 63 05 90 5e 00 ff  00 00 00 00 00 00 00 ff  |r.c..^..........|\n" +
	"00000030  00 00 00 00 00 00 00 ff  11 00 00 00 00 00 00 ff  |................|\n" +
	"00000040  33 00 00 00 00 00 00 01  77 00 00 00 00 00 00 ee  |3.......w.......|\n" +
	"00000050  04 00 00 00 00 00 01 ff  08 00 00 00 00 00 01 ff  |................|\n" +
	"00000060  0a 00 00 00 00 00 01 ff  10 00 00 00 00 00 01 00  |................|\n" +
	"00000070  14 00 00 00 00 00 01 00  80 00 00 00 00 00 01 05  |................|\n"

const writerWantILAStart = "" +
	"00000000  72 c3 63 05 bc dc 00 ff  00 00 00 00 00 00 00 ff  |r.c.............|\n" +
	"00000010  00 00 00 00 00 00 00 ff  11 00 00 00 00 00 00 ff  |................|\n" +
	"00000020  33 00 00 00 00 00 00 01  77 00 00 00 00 00 00 ee  |3.......w.......|\n" +
	"00000030  60 00 00 00 00 00 01 ff  63 00 00 00 00 00 01 ff  |`.......c.......|\n" +
	"00000040  65 00 00 00 00 00 01 ff  68 00 00 00 00 00 01 00  |e.......h.......|\n" +
	"00000050  6c 00 00 00 00 00 01 00  78 00 00 00 00 00 01 05  |l.......x.......|\n" +
	"00000060  52 72 72 53 73 41 61 61  42 62 62 62 43 63 63 63  |RrrSsAaaBbbbCccc|\n" +
	"00000070  63 63 63 63 63 63 31 32                           |cccccc12|\n"

const writerWantILAStartCPageSize4 = "" +
	"00000000  72 c3 63 05 fc 4c 00 ff  00 00 00 00 00 00 00 ff  |r.c..L..........|\n" +
	"00000010  00 00 00 00 00 00 00 ff  11 00 00 00 00 00 00 ff  |................|\n" +
	"00000020  33 00 00 00 00 00 00 01  77 00 00 00 00 00 00 ee  |3.......w.......|\n" +
	"00000030  60 00 00 00 00 00 01 ff  64 00 00 00 00 00 01 ff  |`.......d.......|\n" +
	"00000040  68 00 00 00 00 00 01 ff  6c 00 00 00 00 00 01 00  |h.......l.......|\n" +
	"00000050  70 00 00 00 00 00 01 00  7c 00 00 00 00 00 01 05  |p.......|.......|\n" +
	"00000060  52 72 72 00 53 73 00 00  41 61 61 00 42 62 62 62  |Rrr.Ss..Aaa.Bbbb|\n" +
	"00000070  43 63 63 63 63 63 63 63  63 63 31 32              |Cccccccccc12|\n"

const writerWantILAStartCPageSize128 = "" +
	"00000000  72 c3 63 05 d8 df 00 ff  00 00 00 00 00 00 00 ff  |r.c.............|\n" +
	"00000010  00 00 00 00 00 00 00 ff  11 00 00 00 00 00 00 ff  |................|\n" +
	"00000020  33 00 00 00 00 00 00 01  77 00 00 00 00 00 00 ee  |3.......w.......|\n" +
	"00000030  80 00 00 00 00 00 01 ff  83 00 00 00 00 00 01 ff  |................|\n" +
	"00000040  85 00 00 00 00 00 01 ff  88 00 00 00 00 00 01 00  |................|\n" +
	"00000050  8c 00 00 00 00 00 01 00  98 00 00 00 00 00 01 05  |................|\n" +
	"00000060  00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 00  |................|\n" +
	"00000070  00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 00  |................|\n" +
	"00000080  52 72 72 53 73 41 61 61  42 62 62 62 43 63 63 63  |RrrSsAaaBbbbCccc|\n" +
	"00000090  63 63 63 63 63 63 31 32                           |cccccc12|\n"

func TestWriterILAEndEmpty(t *testing.T) {
	if err := testWriter(IndexLocationAtEnd, nil, 0, true); err != nil {
		t.Fatal(err)
	}
}

func TestWriterILAStartEmpty(t *testing.T) {
	tempFile := &bytes.Buffer{}
	if err := testWriter(IndexLocationAtStart, tempFile, 0, true); err != nil {
		t.Fatal(err)
	}
}

func TestWriterILAEndNoTempFile(t *testing.T) {
	if err := testWriter(IndexLocationAtEnd, nil, 0, false); err != nil {
		t.Fatal(err)
	}
}

func TestWriterILAEndMemTempFile(t *testing.T) {
	tempFile := &bytes.Buffer{}
	if err := testWriter(IndexLocationAtEnd, tempFile, 0, false); err == nil {
		t.Fatal("err: got nil, want non-nil")
	} else if !strings.HasPrefix(err.Error(), "rac: IndexLocationAtEnd requires") {
		t.Fatal(err)
	}
}

func TestWriterILAStartNoTempFile(t *testing.T) {
	if err := testWriter(IndexLocationAtStart, nil, 0, false); err == nil {
		t.Fatal("err: got nil, want non-nil")
	} else if !strings.HasPrefix(err.Error(), "rac: IndexLocationAtStart requires") {
		t.Fatal(err)
	}
}

func TestWriterILAStartMemTempFile(t *testing.T) {
	tempFile := &bytes.Buffer{}
	if err := testWriter(IndexLocationAtStart, tempFile, 0, false); err != nil {
		t.Fatal(err)
	}
}

func TestWriterILAStartRealTempFile(t *testing.T) {
	f, err := ioutil.TempFile("", "rac_test")
	if err != nil {
		t.Fatalf("TempFile: %v", err)
	}
	defer os.Remove(f.Name())
	defer f.Close()

	if err := testWriter(IndexLocationAtStart, f, 0, false); err != nil {
		t.Fatal(err)
	}
}

func TestWriterILAEndCPageSize8(t *testing.T) {
	if err := testWriter(IndexLocationAtEnd, nil, 8, false); err != nil {
		t.Fatal(err)
	}
}

func TestWriterILAStartCPageSize4(t *testing.T) {
	tempFile := &bytes.Buffer{}
	if err := testWriter(IndexLocationAtStart, tempFile, 4, false); err != nil {
		t.Fatal(err)
	}
}

func TestWriterILAStartCPageSize128(t *testing.T) {
	tempFile := &bytes.Buffer{}
	if err := testWriter(IndexLocationAtStart, tempFile, 128, false); err != nil {
		t.Fatal(err)
	}
}

func testWriter(iloc IndexLocation, tempFile io.ReadWriter, cPageSize uint64, empty bool) error {
	buf := &bytes.Buffer{}
	w := &ChunkWriter{
		Writer:        buf,
		IndexLocation: iloc,
		TempFile:      tempFile,
		CPageSize:     cPageSize,
	}

	if !empty {
		// We ignore errors (assigning them to _) from the AddXxx calls. Any
		// non-nil errors are sticky, and should be returned by Close.
		//
		// These {Aa,Bb,Cc} chunks are also used in the reader test.
		res0, _ := w.AddResource([]byte("Rrr"))
		res1, _ := w.AddResource([]byte("Ss"))
		_ = w.AddChunk(0x11, fakeCodec, []byte("Aaa"), 0, 0)
		_ = w.AddChunk(0x22, fakeCodec, []byte("Bbbb"), res0, 0)
		_ = w.AddChunk(0x44, fakeCodec, []byte("Cccccccccc12"), res0, res1)
	}

	if err := w.Close(); err != nil {
		return err
	}
	got := hex.Dump(buf.Bytes())

	want := ""
	switch {
	case empty:
		want = writerWantEmpty
	case (iloc == IndexLocationAtEnd) && (cPageSize == 0):
		want = writerWantILAEnd
	case (iloc == IndexLocationAtEnd) && (cPageSize == 8):
		want = writerWantILAEndCPageSize8
	case (iloc == IndexLocationAtStart) && (cPageSize == 0):
		want = writerWantILAStart
	case (iloc == IndexLocationAtStart) && (cPageSize == 4):
		want = writerWantILAStartCPageSize4
	case (iloc == IndexLocationAtStart) && (cPageSize == 128):
		want = writerWantILAStartCPageSize128
	default:
		return fmt.Errorf("unsupported iloc/cPageSize combination")
	}

	if got != want {
		return fmt.Errorf("\ngot:\n%s\nwant:\n%s", got, want)
	}
	return nil
}

func TestMultiLevelIndex(t *testing.T) {
	buf := &bytes.Buffer{}
	w := &ChunkWriter{
		Writer:        buf,
		IndexLocation: IndexLocationAtStart,
		TempFile:      &bytes.Buffer{},
	}

	// Write 260 chunks with 3 resources. With the current "func gather"
	// algorithm, this results in a root node with two children, both of which
	// are branch nodes. The first branch contains 252 chunks and refers to 3
	// resources (so that its arity is 255). The second branch contains 8
	// chunks and refers to 1 resource (so that its arity is 9).
	xRes := OptResource(0)
	yRes := OptResource(0)
	zRes := OptResource(0)
	primaries := []byte(nil)
	for i := 0; i < 260; i++ {
		secondary := OptResource(0)
		tertiary := OptResource(0)

		switch i {
		case 3:
			xRes, _ = w.AddResource([]byte("XX"))
			yRes, _ = w.AddResource([]byte("YY"))
			secondary = xRes
			tertiary = yRes

		case 4:
			zRes, _ = w.AddResource([]byte("ZZ"))
			secondary = yRes
			tertiary = zRes

		case 259:
			secondary = yRes
		}

		primary := []byte(fmt.Sprintf("p%02x", i&0xFF))
		if i > 255 {
			primary[0] = 'q'
		}
		primaries = append(primaries, primary...)
		_ = w.AddChunk(0x10000, fakeCodec, primary, secondary, tertiary)
	}

	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	encoded := buf.Bytes()
	if got, want := len(encoded), 0x13E2; got != want {
		t.Fatalf("len(encoded): got 0x%X, want 0x%X", got, want)
	}

	gotHexDump := hex.Dump(encoded)
	gotHexDump = "" +
		gotHexDump[0x000*bytesPerHexDumpLine:0x008*bytesPerHexDumpLine] +
		"...\n" +
		gotHexDump[0x080*bytesPerHexDumpLine:0x088*bytesPerHexDumpLine] +
		"...\n" +
		gotHexDump[0x100*bytesPerHexDumpLine:0x110*bytesPerHexDumpLine] +
		"...\n" +
		gotHexDump[0x13C*bytesPerHexDumpLine:]

	const wantHexDump = "" +
		"00000000  72 c3 63 02 17 fa 00 fe  00 00 fc 00 00 00 00 fe  |r.c.............|\n" +
		"00000010  00 00 04 01 00 00 00 ee  30 00 00 00 00 00 04 ff  |........0.......|\n" +
		"00000020  30 10 00 00 00 00 01 ff  e2 13 00 00 00 00 01 02  |0...............|\n" +
		"00000030  72 c3 63 ff 4c be 00 ff  00 00 00 00 00 00 00 ff  |r.c.L...........|\n" +
		"00000040  00 00 00 00 00 00 00 ff  00 00 00 00 00 00 00 ff  |................|\n" +
		"00000050  00 00 01 00 00 00 00 ff  00 00 02 00 00 00 00 ff  |................|\n" +
		"00000060  00 00 03 00 00 00 00 01  00 00 04 00 00 00 00 02  |................|\n" +
		"00000070  00 00 05 00 00 00 00 ff  00 00 06 00 00 00 00 ff  |................|\n" +
		"...\n" +
		"00000800  00 00 f7 00 00 00 00 ff  00 00 f8 00 00 00 00 ff  |................|\n" +
		"00000810  00 00 f9 00 00 00 00 ff  00 00 fa 00 00 00 00 ff  |................|\n" +
		"00000820  00 00 fb 00 00 00 00 ff  00 00 fc 00 00 00 00 ee  |................|\n" +
		"00000830  d9 10 00 00 00 00 01 ff  db 10 00 00 00 00 01 ff  |................|\n" +
		"00000840  e0 10 00 00 00 00 01 ff  d0 10 00 00 00 00 01 ff  |................|\n" +
		"00000850  d3 10 00 00 00 00 01 ff  d6 10 00 00 00 00 01 ff  |................|\n" +
		"00000860  dd 10 00 00 00 00 01 00  e2 10 00 00 00 00 01 01  |................|\n" +
		"00000870  e5 10 00 00 00 00 01 ff  e8 10 00 00 00 00 01 ff  |................|\n" +
		"...\n" +
		"00001000  bb 13 00 00 00 00 01 ff  be 13 00 00 00 00 01 ff  |................|\n" +
		"00001010  c1 13 00 00 00 00 01 ff  c4 13 00 00 00 00 01 ff  |................|\n" +
		"00001020  c7 13 00 00 00 00 01 ff  e2 13 00 00 00 00 01 ff  |................|\n" +
		"00001030  72 c3 63 09 f0 09 00 ff  00 00 00 00 00 00 00 ff  |r.c.............|\n" +
		"00001040  00 00 01 00 00 00 00 ff  00 00 02 00 00 00 00 ff  |................|\n" +
		"00001050  00 00 03 00 00 00 00 ff  00 00 04 00 00 00 00 ff  |................|\n" +
		"00001060  00 00 05 00 00 00 00 ff  00 00 06 00 00 00 00 ff  |................|\n" +
		"00001070  00 00 07 00 00 00 00 ff  00 00 08 00 00 00 00 ee  |................|\n" +
		"00001080  db 10 00 00 00 00 01 ff  ca 13 00 00 00 00 01 ff  |................|\n" +
		"00001090  cd 13 00 00 00 00 01 ff  d0 13 00 00 00 00 01 ff  |................|\n" +
		"000010a0  d3 13 00 00 00 00 01 ff  d6 13 00 00 00 00 01 ff  |................|\n" +
		"000010b0  d9 13 00 00 00 00 01 ff  dc 13 00 00 00 00 01 ff  |................|\n" +
		"000010c0  df 13 00 00 00 00 01 00  e2 13 00 00 00 00 01 09  |................|\n" +
		"000010d0  70 30 30 70 30 31 70 30  32 58 58 59 59 70 30 33  |p00p01p02XXYYp03|\n" +
		"000010e0  5a 5a 70 30 34 70 30 35  70 30 36 70 30 37 70 30  |ZZp04p05p06p07p0|\n" +
		"000010f0  38 70 30 39 70 30 61 70  30 62 70 30 63 70 30 64  |8p09p0ap0bp0cp0d|\n" +
		"...\n" +
		"000013c0  38 70 66 39 70 66 61 70  66 62 70 66 63 70 66 64  |8pf9pfapfbpfcpfd|\n" +
		"000013d0  70 66 65 70 66 66 71 30  30 71 30 31 71 30 32 71  |pfepffq00q01q02q|\n" +
		"000013e0  30 33                                             |03|\n"

	if gotHexDump != wantHexDump {
		t.Fatalf("\ngot:\n%s\nwant:\n%s", gotHexDump, wantHexDump)
	}

	r := &ChunkReader{
		ReadSeeker: bytes.NewReader(encoded),
	}
	gotPrimaries := []byte(nil)
	for {
		c, err := r.NextChunk()
		if err == io.EOF {
			break
		} else if err != nil {
			t.Fatalf("NextChunk: %v", err)
		}
		p0, p1 := c.CPrimary[0], c.CPrimary[1]
		if (0 <= p0) && (p0 <= p1) && (p1 <= int64(len(encoded))) {
			primary := encoded[p0:p1]
			if len(primary) > 3 {
				primary = primary[:3]
			}
			gotPrimaries = append(gotPrimaries, primary...)
		}
	}
	if !bytes.Equal(gotPrimaries, primaries) {
		t.Fatalf("\ngot:\n%s\nwant:\n%s", gotPrimaries, primaries)
	}
}

func TestWriter1000Chunks(t *testing.T) {
loop:
	for i := 0; i < 2; i++ {
		buf := &bytes.Buffer{}
		w := &ChunkWriter{
			Writer: buf,
		}
		if i > 0 {
			w.IndexLocation = IndexLocationAtStart
			w.TempFile = &bytes.Buffer{}
		}

		data := make([]byte, 1)
		res, _ := w.AddResource(data)
		for i := 0; i < 1000; i++ {
			if i == 2*255 {
				_ = w.AddChunk(1, fakeCodec, data, res, 0)
			} else {
				_ = w.AddChunk(1, fakeCodec, data, 0, 0)
			}
		}
		if err := w.Close(); err != nil {
			t.Errorf("i=%d: Close: %v", i, err)
			continue loop
		}

		encoded := buf.Bytes()
		r := &ChunkReader{
			ReadSeeker: bytes.NewReader(encoded),
		}
		for n := 0; ; n++ {
			if _, err := r.NextChunk(); err == io.EOF {
				if n != 1000 {
					t.Errorf("i=%d: number of chunks: got %d, want %d", i, n, 1000)
					continue loop
				}
				break
			} else if err != nil {
				t.Errorf("i=%d: NextChunk: %v", i, err)
				continue loop
			}
		}
	}
}

func printChunks(chunks []Chunk) string {
	ss := make([]string, len(chunks))
	for i, c := range chunks {
		ss[i] = fmt.Sprintf("D:%#x, C0:%#x, C1:%#x, C2:%#x, S:%#x, T:%#x, C:%#x",
			c.DRange, c.CPrimary, c.CSecondary, c.CTertiary, c.STag, c.TTag, c.Codec)
	}
	return strings.Join(ss, "\n")
}

func TestChunkReader(t *testing.T) {
	testCases := []struct {
		name       string
		compressed []byte
	}{
		{"Empty", undoHexDump(writerWantEmpty)},
		{"ILAEnd", undoHexDump(writerWantILAEnd)},
		{"ILAEndCPageSize8", undoHexDump(writerWantILAEndCPageSize8)},
		{"ILAStart", undoHexDump(writerWantILAStart)},
		{"ILAStartCPageSize4", undoHexDump(writerWantILAStartCPageSize4)},
		{"ILAStartCPageSize128", undoHexDump(writerWantILAStartCPageSize128)},
	}

loop:
	for _, tc := range testCases {
		snippet := func(r Range) string {
			if r.Empty() {
				return ""
			}
			if r[1] > int64(len(tc.compressed)) {
				return "CRange goes beyond COffMax"
			}
			s := tc.compressed[r[0]:r[1]]
			if len(s) > 2 {
				return string(s[:2]) + "..."
			}
			return string(s)
		}

		wantDecompressedSize := int64(0)
		wantDescription := ""
		if tc.name != "Empty" {
			// These {Aa,Bb,Cc} chunks are also used in the writer test.
			wantDecompressedSize = 0x11 + 0x22 + 0x44
			wantDescription = `` +
				`DRangeSize:0x11, C0:"Aa...", C1:"", C2:""` + "\n" +
				`DRangeSize:0x22, C0:"Bb...", C1:"Rr...", C2:""` + "\n" +
				`DRangeSize:0x44, C0:"Cc...", C1:"Rr...", C2:"Ss..."`
		}

		r := &ChunkReader{
			ReadSeeker: bytes.NewReader(tc.compressed),
		}

		if gotDecompressedSize, err := r.DecompressedSize(); err != nil {
			t.Errorf("%q test case: %v", tc.name, err)
			continue loop
		} else if gotDecompressedSize != wantDecompressedSize {
			t.Errorf("%q test case: DecompressedSize: got %d, want %d",
				tc.name, gotDecompressedSize, wantDecompressedSize)
			continue loop
		}

		gotChunks := []Chunk(nil)
		description := &bytes.Buffer{}
		prevDRange1 := int64(0)
		for {
			c, err := r.NextChunk()
			if err == io.EOF {
				break
			} else if err != nil {
				t.Errorf("%q test case: NextChunk: %v", tc.name, err)
				continue loop
			}

			if c.DRange[0] != prevDRange1 {
				t.Errorf("%q test case: NextChunk: DRange[0]: got %d, want %d",
					tc.name, c.DRange[0], prevDRange1)
				continue loop
			}
			prevDRange1 = c.DRange[1]

			gotChunks = append(gotChunks, c)
			if description.Len() > 0 {
				description.WriteByte('\n')
			}
			fmt.Fprintf(description, "DRangeSize:0x%X, C0:%q, C1:%q, C2:%q",
				c.DRange.Size(), snippet(c.CPrimary), snippet(c.CSecondary), snippet(c.CTertiary))
		}

		if tc.name == "Empty" {
			if len(gotChunks) != 0 {
				t.Errorf("%q test case: NextChunk: got non-empty, want empty", tc.name)
			}
			continue loop
		}

		gotDescription := description.String()
		if gotDescription != wantDescription {
			t.Errorf("%q test case: NextChunk:\n    got\n%s\n    which is\n%s\n    want\n%s",
				tc.name, printChunks(gotChunks), gotDescription, wantDescription)
			continue loop
		}

		// NextChunk should return io.EOF.
		if _, err := r.NextChunk(); err != io.EOF {
			t.Errorf("%q test case: NextChunk: got %v, want io.EOF", tc.name, err)
			continue loop
		}

		if err := r.SeekToChunkContaining(0x30); err != nil {
			t.Errorf("%q test case: SeekToChunkContaining: %v", tc.name, err)
			continue loop
		}

		// NextChunk should return the "Bb..." chunk.
		if c, err := r.NextChunk(); err != nil {
			t.Errorf("%q test case: NextChunk: %v", tc.name, err)
			continue loop
		} else if got, want := snippet(c.CPrimary), "Bb..."; got != want {
			t.Errorf("%q test case: NextChunk: got %q, want %q", tc.name, got, want)
			continue loop
		}
	}
}

func TestReaderEmpty(t *testing.T) {
	got, err := ioutil.ReadAll(&Reader{
		ReadSeeker: bytes.NewReader(undoHexDump(writerWantEmpty)),
	})
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %q, want %q", got, []byte(nil))
	}
}

func TestReaderZeroes(t *testing.T) {
	const dSize = 7
	buf := &bytes.Buffer{}
	w := &ChunkWriter{
		Writer: buf,
	}
	if err := w.AddChunk(dSize, CodecZeroes, nil, 0, 0); err != nil {
		t.Fatalf("AddChunk: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	got, err := ioutil.ReadAll(&Reader{
		ReadSeeker: bytes.NewReader(buf.Bytes()),
	})
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	want := make([]byte, dSize)
	if !bytes.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}