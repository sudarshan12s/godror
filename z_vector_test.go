// Copyright 2018, 2020 The Godror Authors
//
//
// SPDX-License-Identifier: UPL-1.0 OR Apache-2.0

package godror_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	godror "github.com/godror/godror"
)

//var birthdate, _ = time.Parse(time.UnixDate, "Wed Feb 25 11:06:39 PST 1990")

func TestReadWriteVector(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(testContext("ReadWriteVector"), 30*time.Second)
	defer cancel()
	conn, err := testDb.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	tbl := "test_personcollection_vector" + tblSuffix
	conn.ExecContext(ctx, "DROP TABLE "+tbl)
	_, err = conn.ExecContext(ctx,
		"CREATE TABLE "+tbl+" (id NUMBER(6), embedding VECTOR)", //nolint:gas
	)
	if err != nil {
		if errIs(err, 902, "invalid datatype") {
			t.Skip(err)
		}
		t.Fatal(err)
	}
	t.Logf(" JSON Document table  %q: ", tbl)

	defer testDb.Exec(
		"DROP TABLE " + tbl, //nolint:gas
	)

	stmt, err := conn.PrepareContext(ctx,
		"INSERT INTO "+tbl+" (id, embedding) VALUES (:1, :2)", //nolint:gas
	)
	if err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()
	//var travelTime time.Duration = 5*time.Hour + 21*time.Minute + 10*time.Millisecond + 20*time.Nanosecond
	var embedding [3]float64

	embedding[0] = 1.1
	embedding[1] = 2.2
	embedding[2] = 3.3
	// values for batch insert
	const num = 1
	ids := make([]godror.Number, num)
	docs := make([]godror.Vector[float32], num)
	for i := range ids {
		docs[i] = godror.Vector[float32]{values: embedding}
		ids[i] = godror.Number(strconv.Itoa(i))
	}

	// value for last row to simulate single row insert
	lastIndex := godror.Number(strconv.Itoa(num))
	lastJSONDoc := godror.Vector{values: embedding}

	for tN, tC := range []struct {
		ID        interface{}
		EMBEDDING godror.Vector
	}{
		{EMBEDDING: docs, ID: ids},
		{EMBEDDING: lastJSONDoc, ID: lastIndex},
	} {
		if _, err = stmt.ExecContext(ctx, tC.ID, tC.EMBEDDING); err != nil {
			t.Errorf("%d/1. (%v): %v", tN, tC.EMBEDDING, err)
			continue
		}

		var rows *sql.Rows
		rows, err = conn.QueryContext(ctx,
			"SELECT id, embedding FROM "+tbl) //nolint:gas
		if err != nil {
			t.Errorf("%d/3. %v", tN, err)
			continue
		}
		var id interface{}
		var jsondoc godror.Vector
		var ok bool
		for rows.Next() {
			if err = rows.Scan(&id, &jsondoc); err != nil {
				rows.Close()
				t.Errorf("%d/3. scan: %v", tN, err)
				continue
			}

			if err != nil {
				t.Errorf("%d. %v", id, err)
			} else {
				t.Logf("%d. JSON Document read %q: ", id, jsondoc)

				var gotmap []float64
				v, err := jsondoc.GetValues()
				if err != nil {
					t.Errorf("%d. %v", id, err)
				}
				if gotmap, ok = v.([]float64); !ok {
					t.Errorf("%d. %T is not JSONObject ", id, v)
				}
				eq := reflect.DeepEqual(embedding, gotmap)
				if !eq {
					t.Errorf("Got %+v, wanted %+v", gotmap, embedding)
				}
			}

		}
		rows.Close()
	}
}
