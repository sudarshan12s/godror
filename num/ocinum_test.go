// Copyright 2020 The Godror Authors
// Copyright 2016 Tamás Gulácsi
//
//
// SPDX-License-Identifier: UPL-1.0 OR Apache-2.0

//lint:file-ignore ST1018 Already generated, hard to convert.

package num

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var testNums = []struct {
	await string
	num   []byte
}{
	{"0", []byte{128}},
	{"1", []byte{193, 2}},
	{"10", []byte{193, 11}},
	{"100", []byte{194, 2}},
	{"1000", []byte{194, 11}},
	{"10000", []byte{195, 2}},
	{"123", []byte{194, 2, 24}},
	{"12.3", []byte{193, 13, 31}},
	{"1.23", []byte{193, 2, 24}},

	/*

	   SQL> select dump(12345), dump(1234.5), dump(123.45), dump(12.345), dump(1.2345), dump(0.12345), dump(0.012345) from dual;

	   DUMP(12345)              DUMP(1234.5)              DUMP(123.45)
	   ------------------------ ------------------------- ------------------------
	   DUMP(12.345)              DUMP(1.2345)             DUMP(0.12345)
	   ------------------------- ------------------------ -------------------------
	   DUMP(0.012345)
	   ------------------------
	   Typ=2 Len=4: 195,2,24,46 Typ=2 Len=4: 194,13,35,51 Typ=2 Len=4: 194,2,24,46
	   Typ=2 Len=4: 193,13,35,51 Typ=2 Len=4: 193,2,24,46 Typ=2 Len=4: 192,13,35,51
	   Typ=2 Len=4: 192,2,24,46
	*/
	{"12345", []byte{195, 2, 24, 46}},
	{"1234.5", []byte{194, 13, 35, 51}},
	{"123.45", []byte{194, 2, 24, 46}},
	{"12.345", []byte{193, 13, 35, 51}},
	{"1.2345", []byte{193, 2, 24, 46}},
	{"0.12345", []byte{192, 13, 35, 51}},
	{"0.012345", []byte{192, 2, 24, 46}},

	{"1", []byte{193, 2}},
	{"-1", []byte{62, 100, 102}},
	{"12", []byte{193, 13}},
	{"20", []byte{193, 21}},
	{"-12", []byte{62, 89, 102}},
	{"123", []byte{194, 2, 24}},
	{"-123", []byte{61, 100, 78, 102}},
	{"123456789012345678901234567890123456789", []byte{212, 2, 24, 46, 68, 90, 2, 24, 46, 68, 90, 2, 24, 46, 68, 90, 2, 24, 46, 68, 90}},
	{"-123456789012345678901234567890123456789", []byte{43, 100, 78, 56, 34, 12, 100, 78, 56, 34, 12, 100, 78, 56, 34, 12, 100, 78, 56, 34, 12}},

	{"1000", []byte{194, 11}},
	{"-1000", []byte{61, 91, 102}},
	{"0.1", []byte{192, 11}},
	{"-0.1", []byte{63, 91, 102}},
	{"0.01", []byte{192, 2}},
	{"-0.01", []byte{63, 100, 102}},
	{"0.12", []byte{192, 13}},
	{"-0.12", []byte{63, 89, 102}},
	{"0.012", []byte{192, 2, 21}},
	{"-0.012", []byte{63, 100, 81, 102}},

	{`66000`, []byte{195, 7, 61}},
	{`3999900`, []byte{196, 4, 100, 100}},
	{`509090007050906000600`, []byte{203, 6, 10, 10, 1, 8, 6, 10, 7, 1, 7}},
	{`600066000`, []byte{197, 7, 1, 7, 61}},
	{"-11166232058078251449.063252477", []byte{53, 90, 85, 39, 69, 96, 21, 23, 76, 87, 52, 95, 69, 49, 54, 31, 102}},
	{"-9402004353104906.474368202171", []byte{55, 7, 99, 101, 58, 48, 91, 52, 95, 54, 58, 33, 81, 80, 30, 102}},

	{"-23452342342423423423423.1234567890123456", []byte{51, 99, 67, 49, 67, 78, 59, 59, 67, 78, 59, 67, 78, 89, 67, 45, 23, 11, 89, 67, 45}},
}

func TestOCINumPrint(t *testing.T) {
	only := os.Getenv("ONLY")
	var b []byte
	for eltNum, elt := range testNums {
		if only != "" && only != elt.await {
			continue
		}
		b = OCINum(elt.num).Print(b)
		if !bytes.Equal(b, []byte(elt.await)) {
			t.Errorf("%d. % v\ngot\n\t%s (% v)\nawaited\n\t%s (% v).", eltNum, elt.num, b, b, elt.await, []byte(elt.await))
		}
	}
}

func TestOCINumSet(t *testing.T) {
	only := os.Getenv("ONLY")
	var num OCINum
	for eltNum, elt := range testNums {
		if only != "" && only != elt.await {
			continue
		}
		if err := num.SetString(elt.await); err != nil {
			t.Errorf("%d. %s: %v", eltNum, elt.await, err)
			continue
		}
		if !bytes.Equal(num, elt.num) {
			t.Errorf("%d. %s:\ngot\n\t%v\nawaited\n\t%v", eltNum, elt.await, []byte(num), elt.num)
		}
	}
}

func TestOCINumSetString(t *testing.T) {
	only := os.Getenv("ONLY")
	var a [22]byte
	for _, group := range []struct {
		bad   bool
		cases []string
	}{
		{false, setStringCasesGood},
		{true, setStringCasesBad},
	} {
		for eltNum, elt := range group.cases {
			if only != "" && only != elt {
				continue
			}
			n := OCINum(a[:0])
			err := n.SetString(elt)
			if group.bad && err == nil {
				if strings.TrimSpace(elt) != "" {
					t.Errorf("%d. no error for %q!", eltNum, elt)
				}
				continue
			} else if !group.bad && err != nil {
				t.Errorf("%d. %q: %v", eltNum, elt, err)
				continue
			}
			if group.bad {
				continue
			}
			if got := n.String(); got != strings.TrimSpace(elt) {
				t.Errorf("%d. got %q, awaited %q (%v).", eltNum, got, elt, []byte(n))
			}
		}
	}
}

func TestDeCompose(t *testing.T) {
	p := make([]byte, 38)
	var n OCINum
	for i, s := range []string{
		"0",
		"1",
		"-2",
		"3.14",
		"-3.14",
		"1000",
		"3.456789",
		"0.01",
		"-0.09",
		"-0.89",
		"0.0000000001",
		"1.0000000002",
		"12345678901234567890123456789012345678",
		"120056789012005678901200567890100456780",
	} {
		if err := n.SetString(s); err != nil {
			t.Fatalf("%d. %q: %+v", i, s, err)
		}

		form, negative, coefficient, exponent := n.Decompose(p[:0])
		if want := s[0] == '-'; want != negative {
			t.Errorf("%d. Decompose(%q) got negative=%t, wanted %t", i, s, negative, want)
		}
		t.Logf("%d. %q: form=%d negative=%t exponent=%d coefficient=%v orig=%v", i, s, form, negative, exponent, coefficient, []byte(n))
		var m OCINum
		if err := m.Compose(form, negative, coefficient, exponent); err != nil {
			t.Errorf("%d. cannot compose %c/%t/% x/%d from %q", i, form, negative, coefficient, exponent, s)
		}
		t.Logf("%d. m=%v", i, []byte(m))
		if got := m.String(); got != s {
			t.Errorf("%d. got %q wanted %q", i, got, s)
		}
	}
}

func TestPrintCorpus(t *testing.T) {
	hsh := sha1.New()
	var h []byte
	os.MkdirAll("corpus", 0750)
	for _, ss := range [][]string{setStringCasesGood, setStringCasesBad} {
		for _, s := range ss {
			hsh.Reset()
			hsh.Write([]byte(s))
			h = hsh.Sum(h[:0])
			fn := filepath.Join("corpus", hex.EncodeToString(h))
			if fi, err := os.Stat(fn); err == nil {
				if fi.Size() != int64(len(s)) {
					os.Remove(fn)
				}
				continue
			}
			if err := ioutil.WriteFile(fn, []byte(s), 0444); err != nil {
				t.Fatal(err)
			}
		}
	}
}

var setStringCasesGood = []string{
	`9`,
	`2000000000000000000`,
	`536743164`,
	`20000`,
	`0`,
	`200000000`,
	`-9402004353104906.474368202171`,
	`74`,
	`2000000`,
	`20`,
	`200`,
	`20000000000000000`,
	`94`,
	`435310490647436820217`,
	`53`,
	`2`,
	`74`,
	`2000000000000000000000000000000000`,
	`-129`,
	`1`,
	`6 `,

	`907050906`,
	`9　`,
	`66000`,
	`142108547152020037174224853515625`,
	` 2 `,
	`745580596923828125`,
	`600`,
	`6005000000000000000000000000000000000`,
	`0 `,
	`3444089209850062616169452667236328125`,
	`-102`,
	`4 `,
	` 9 `,
	`6000000`,
	`60705090600066000`,
	`6000000000`,
	`7810000000000000000`,
	`-506210721134567`,
	`-2`,
	`390625`,
	`5　　　　　　　　　　　　　　　　　`,
	`90707050906050906`,
	`6 `,
	` 4`,
	`6058068096923806600`,
	`9090906`,
	` 0`,
	`600055756156289135105907917022705078125`,
	`9　　`,
	`600596923806600`,
	`-11166232058465661287307739257812547`,
	`2 `,
	`60000000000000000000000000000000006`,
	`0　　　　　　　　　`,
	`39900`,
	`7450580596923828125`,
	`3390909062`,
	`-11166232058078251449.063252477`,
	`0.6`,
	`390`,
}

var setStringCasesBad = []string{
	"-",
	``,
	`đľżżđľżżđ˝ż˝đ˝ż˝`,
	``,
	`"""""`,
	`Ă Ă Ă Ă Ă `,
	`ŢžďÓďďďżż`,
	`đĽĽđĽĽđĽđĽđĽ`,
	` `,
	`ż˝żăăăăăăă`,
	`ŢÓďÓďďďÍć`,
	`đľ`,
	`'"`, `ŢÓďÓďďďŰŮÂďÓďďďďÍć`,
	` Â Â Â Â `,
	`î>`,
	`ăăăăăăăăăăă`,
	`ÓşÓşÓşÓşÓşÓş`,
	`˝Ó`,
	`@ @`,
	`@ @`,
	`żż˝ďżÉďżďďďżÉďżďďďżÉż˝żż˝ďżÉďżďďż˝ż˝żď*˝żď\˝żżżżď˝ďżÉż˝żż˝ďżÉďżďďďżÉż˝ďď`,
	`ďż˝ďż˝ďżżď˝żď˝żďż˝ďż˝ď˝żďż˝ďż˝ď˝żďż˝ďż˝ďżż`,
	`Ď`,
	`đĽ`,
	``,
	`đľ`,
	`-e`,
	`ß Ă Â `,
	`˙ Ą`,
	`˝`,
	`<`,
	``,
	`"`,
	``,
	``,
	`đĽżđĽżđĽżđĽżđ˝˝đĽ˝`,
	`đľżżđ˝ż˝`,
	`ďżÓďżĆ`,
	`ďżÓďďďďżĆ`,
	` `,
	`ăăăăăăăă`,
	`'"`, `  `,
	`5"""""""""""""""""`,
	`too few operands for _ormat `,
	`ďżÓďżÉďżďďďżÉďżďďď*˝ż˝żżżżď˝ďżÉż˝żż˝ďżÉďżÉż˝żż˝ďżÉďżďďż˝ż˝żď*˝żď\˝żżżżď˝ďżÉż˝żż˝ďżÉďżďďďżÉż˝ďďď`,
	``,
	`'"`, `5"`,
	``,
	`â­`,
	`Éżďżď˝ď˝ď˝ďżż`,
	`ďżÓďďďďŹďďżĆ`,
	`Ó`,
	`)It¸`,
	`'"`, `"""`,
	`Éżď˝żďż˝ďż˝ďż˝ďżż`,
	`ďżÓďďďďŹďżĆ`,
	`Ă Ă Ă Ă Ă Ă Ă Ă Ă `,
	` ź`, `ďżîżÉďżďď˝ďżÉďżÉďżÉďżÉďżďďďďďżÉďżďďďďżďďďżÉďżďď˝ďżÉď˝ďżÉďżÉďżÉďżÉďżďďď˝ďżÉďżÉďżÉďżÉďżďďď˝ďżÉďżÉďżďďďżÉż`,
	`@ @`,
	`ŢžÓż`,
	`á˝˝`,
	`-.-`,
	`n`,
	`đĽżđĽ˝`,
	` o `,
	`Ă Ă ĂĂ Ă ĂĂ Ă Ă Ă Ă ĂľĂ Ă Ă Ă Ă Ă Ă Ă `,
	`ăăăăăăăăăă`,
	``,
	`đľ`,
	`ăăăăăăăăă`,
	`  `,
	`054175252231364715157010273365424.-0xaC4bc3bBFEE733c17Cb23c7B4E9`,
	`ăă`,
	``,
	`đľđľđż˝đľđ`,
	``,
	`ďîĄ÷˝żďżďî˝ďżf˝ď`,
	`Ă Ă Ă Ă Ă Ă Ă `,
	`ŢžÓşÓş`,
	``,
	`@ `,
	`ďż˝ďż˝ď˝żď˝żď˝żďż˝ďż˝ďż˝ď˝żď˝żďż˝ďż˝ď˝żďż˝ďż˝ď˝żďż˝ęż˝ď˝żďż˝ďż˝ď˝żďż˝ď˝żď˝żďż˝ďż˝ď˝żďż˝ďż˝ď˝żďż˝ďż˝`,
	``,
	`@ `,
	`ŢÓďÓďďďżż`,
	`ďżżďżżďżšďżżďżż`,
	` @`,
	`nfĂ Â Â Â `,
	`9u 536743164˙˙˙`,
	`đľđż˝ďđľđ`,
	`˝Ó`,
	`ăăăă`,
	`ďż˝ďż˝ďż˝ďż˝ďż˝Ýż`,
	`ďż˝ďż˝ď˝żď˝żďż˝ďż˝ď˝żď˝żďż˝ďż˝ď˝żďż˝ďż˝ď˝żďż˝ęż˝ď˝żďż˝ďż˝ď˝żďż˝ď˝żď˝żďż˝ďż˝ď˝żďż˝ďż˝ď˝żďż˝ďż˝`,
	`đľ0`,
	`ÓşÓşÓşÓşÓşÓşÓşÓşÓşÓş`,
	`ďż˝ďż˝ďż˝ďż˝ďż˝`,
	`Ă ÂÂ Â `,
	`đĽĽđĽĽđ đĽđĽĽđĽĽđ đĽđĽĽđĽĽđĽĽđ đĽđĽĽđĽĽđ đĽđĽ`,
	`đľżżđľżżđľżżđľżżđ˝ż˝đż˝đ˝ż˝đ˝żżđľżżđľżżđľżżđ˝ż˝đż˝đ˝ż˝`,
	``,
	`ďż˝ďż˝ď˝żď˝żďż˝ďż˝ď˝żďż˝ďż˝ď˝żďż˝ď˝żď˝żďż˝ďż˝ď˝żďż˝ďż˝ď˝żďż˝ďż˝`,
	`ÓşÓşÓşÓşÓşÓşÓşÓşÓşÓşÓşÓşÓşÓşÓşÓş`,
	` `, `Aďż ˝ďż˝ďż˝ďż˝3ďż˝`,
	` `, `'"`, ` `,
	`ăăăă`,
	`o `,
	`ďżÓîżÉďżďďďżÉďżďďď*˝ż˝żżżżÉďżďďďżÉďżďď˝ďżÉż˝żżżď˝ďżÉż˝żż˝ďżÉďżÉż˝żż˝ďżÉďżďďż˝ż˝żď*˝żď\źżżżżżď˝ďżÉż˝żż˝ďżÉďżÉż˝żż˝ďżÉďżďďż˝ż˝żď*˝żď\źżżżżď˝ďżÉż˝żż˝ďżÉďżďďďżÉż˝ďďď`,
	`@ `,
	`îżď˝ďżďżďżďżďżďżďżďżďżď˝ďżďżďżďżďżďżďżďżďżď˝ďżďżďżď˝ďżď˝ďżďżďżď˝ďżďżďżďżďżď˝ďżďżďżďżďżď˝ďżď˝ďżďżďżďżďżď˝ďżďżďżďżďżď˝ďżďżďżďżż`,
	``,
	`ŢÓďinputdoes no˝żďtJh formatÓWď(ď`,
	`ďżÓîżÉďżďďďżÉďżďďď*˝ż˝żżżżÉďżďďďżÉďżďď˝ďżÉż˝żżżď˝ďżÉżżż˝ďżÉďżÉż˝żż˝ďżÉďżÉż˝żż˝ďżÉďżż˝żď*˝ż˝żż˝ďżÉďżďďż˝ż˝żď*˝żď\źżżżżżď˝ďżÉż˝żż˝ďżÉďżÉż˝żż˝ďżÉďżż˝żď*˝żżď˝ďżÉż˝żż˝ďżÉďżďďďżÉż˝ďďď`,
	`ďżďťxďŹďżĆ`,
	`˙ ˝`,
	`đĽĽđĽĽđĽĽđ đĽđĽ`,
	`Éżż˝ďżÉďżď˝żďďżÉďżď˝żďż˝ďżÉďż˝żż˝ďżÉďżď˝żďż˝ż˝ďżÉďż˝żż˝ďżÉďżď˝żďż˝ďżÉďż˝ďďżż`,
	``,
	`čy`,
	` Ţ `,
	`0123456789abcdefghijklmnopqrstuvwxy@<M($Fz@`,
	`Ă Â Â `,
	`ŢÓďÓďÓďďďŰŮÂďÓďďďďÍÓďďďŰŮÂďÓďďďďÍl`,
	`đľżżđľżżđľżżđľżżđ˝ż˝đ˝ż˝đ˝ż˝`,
	`@`,
	`đĽĽđĽĽđĽĽđĽĽđ đĽđĽĽđĽĽđ đĽđĽ`,
	``,
	`Â Â Â Â Â `,
	`ăăăăăăăăăăăăăăăăăăăăăăăăăăăăăăăăăăăă`,
	`ăăăăăăăăăăăăăă`,
	`ăăăăăăăăăăăăăăăăăă`,
	`)It¸`,
	`đđÝ`,
	` ˝`,
	`ăăăăăăăăăăăăăăăăăăăăăăăăăăăăăăăăă`,
	`˙ ˝`,
	`'""`, `đĽżđĽżđĽ˝đĽ˝`,
	`ăżîżď˝ďżďżďżďżďżďżďżďżďżď˝ďżďżďżďżďżďżďżďżďżď˝ďżďżďżď˝ďżď˝ďżďżďżď˝ďżďżďżďżďżď˝ďżďżďżďżďżď˝ďżď˝ďżďżďżďżďżď˝ďżďżďżďżďżď˝ďżďżďżďżÉ`,
	`ÓşÓşÓşÓşÓşÓşÓşÓşÓşÓşÓşÓşÓşÓşÓşÓşÓş`,
	`ĂżîżÉďżďď˝ďżÉďżÉďżÉďżÉďżďďďďďżÉďżďďďďżďďďżÉď˝ďżďżÉďżÉďżÉďżďďďďďżÉďżďďďďżďďďżÉď˝ďżÉďżďżďď˝ďżÉď˝ďżÉďżÉďżÉ˙ďďď˝ďżÉďżÉďżÉďżČďżďďď˝ďżÉďżÉďżďÉďżďżďď˝ďżÉď˝ďżÉďżÉďżÉďżÉďżďďď˝ďżÉďżÉďżÉďżČďżďďď˝ďżÉďżÉďżďďďżÉż`,
	`ăăăăăăăăăăăăăăăăăăăăăăăăăăăăăăăăăăăăă`,
	`Â`,
	`żďîĄ÷˝żďżďî˝ďżf˝ď`,
	`Aďż ˝ďż˝ż˝`,
	``,
	` @`,
	`đľżżđľżżđľżżđľżżń˝ż˝ń˝ż˝đľżżđľżżđ˝ż˝đľżżđľżżđľżżń˝ż˝ń˝ż˝đľżżđľżżđ˝ż˝`,
	`'"`, `-0xbafD8aae3Df5b9Bef1530xCc8EBf357FEdaCfCF4CdeBEEbaE47fb5Bc691.-0xd-0534420.-8953`,
	`5"""""""""`,
	`ďż˝ďż˝ďż˝ďż˝ďż˝ď˝˝`,
	`Óż`,
	`Ă Ă `,
	`đĽ`,
	`536743164˙˙˙`,
	`ÓşÓşÓşÓşÓş`,
	`ăăăăăăăă`,
	`ďżÓż`,
	`Óďďďďż`,
	` `,
	`˙ ˝`,
	`@ @`,
	``,
	`żż˝ż˝`,
	``,
	`đĽĽđĽ`,
	`too few operands for _ormat @%`,
	`@ `,
	``,
	`đĽżđĽżđĽżđ˝˝đĽżđĽżđĽżđĽżđ˝˝đĽ˝`,
	` Ó`,
	`"`,
	`ÓşÓşÓşÓşÓşÓşÓşÓ˛ÓşÓşÓşÓşÓşÓşÓşÓşĎşÓşÓşÓşÓşÓşÓşĎşÓşÓşÓşÓşÓşÓşÓşÓşÓşÓşÓşÓşÓşÓşÓşÓşÓşÓş`,
	``,
	``,
	`ÓşÓşÓşÓşÓşÓşÓşÓş`,
	``,
	`ÓşÓşÓşÓşÓşÓşÓşÓ˛ÓşÓşÓşÓşÓşÓşÓşÓşĎşÓşÓşÓşÓşÓşÓşÓşÓşÓşÓşÓşÓşÓş`,
	`áż˝`,
	`<`,
	`đĽżđĽżđĽżđĽżđĽżđ˝˝đĽżđĽżđĽżđĽżđ˝˝đĽżđĽżđĽżđĽżđ˝˝đĽ˝`,
	`ăăăăăăăăăăăăăăăă`,
	`ţż˝˝ż˝żż˝˝ż˝`,
	``,
	`Â Ă`,
	``,
	`ăăăăăăăăăăăăăăăăăăăăăăăăăăăăăăăăă`,
	`ďĆ`,
	`żďăăă`,
	`ăăăăăăăăăăăăăăăăăăăăăăăă`,
	`Ă Â Â Â `,
	`ďżżďżżď˝ďżżâż˝ďżż`,
	`Â Â Â `,
	`Ă`,
	`ÓşÓşÓşÓşÓşÓşÓşÓ˛ÓşÓşÓşÓşÓşÓşÓşÓşĎşÓşÓşÓşÓşÓşÓşÓşĎşÓşÓşÓşÓşÓşÓşĎşÓşÓşÓşÓşÓşÓşÓşÓşÓşÓşÓşÓşÓşÓşÓşÓşÓşÓşÓşÓşÓ˛ÓşÓşÓşÓşÓşÓşÓşÓşÓşÓşÓşÓş`,
	`ďżżďżżďżżďżżďżšďżżďżšďżżďżż`,
	`˙ ˝`,
	`đĽđĽĽđĽ`,
	`ÓşÓşÓş`,
	`ĂÂ Â `,
	`Â Â Â Â Â Â Â Â Â `,
	`˝żďăă`,
	`đľđ`,
	`Ţ `,
	`@ @`,
}