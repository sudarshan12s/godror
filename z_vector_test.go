// Copyright 2025 The Godror Authors
//
//
// SPDX-License-Identifier: UPL-1.0 OR Apache-2.0

package godror_test

import (
	"context"
	"database/sql"
	_ "errors"
	_ "fmt"
	"reflect"
	"strconv"
	_ "strings"
	"testing"
	"time"

	godror "github.com/godror/godror"
)

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
		"CREATE TABLE "+tbl+" (id NUMBER(6), image_vector VECTOR, graph_vector VECTOR(5, float32, SPARSE) )", //nolint:gas
	)
	if err != nil {
		if errIs(err, 902, "invalid datatype") {
			t.Skip(err)
		}
		t.Fatal(err)
	}
	t.Logf(" Vector table  %q: ", tbl)

	defer testDb.Exec(
		"DROP TABLE " + tbl, //nolint:gas
	)

	stmt, err := conn.PrepareContext(ctx,
		"INSERT INTO "+tbl+" (id, image_vector, graph_vector) VALUES (:1, :2, :3)", //nolint:gas
	)
	if err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()
	var embedding = []float32{1.1, 2.2, 3.3}

	var graph_vector godror.Vector[float32] = godror.Vector[float32]{
		Indices:    []uint32{0, 2, 3},
		Dimensions: 5,
		Values:     []float32{0.5, 1.2, -0.9},
		IsSparse:   true,
	}

	// values for batch insert
	const num = 3
	ids := make([]godror.Number, num)
	images := make([]godror.Vector[float32], num)
	nodes := make([]godror.Vector[float32], num)
	for i := range ids {
		nodes[i] = graph_vector
		images[i] = godror.Vector[float32]{
			Values: embedding,
		}
		ids[i] = godror.Number(strconv.Itoa(i))
	}

	// value for last row to simulate single row insert
	lastIndex := godror.Number(strconv.Itoa(num))
	lastImage := godror.Vector[float32]{Values: embedding}
	lastNode := graph_vector

	for tN, tC := range []struct {
		ID           interface{}
		IMAGE_VECTOR interface{}
		GRAPH_VECTOR interface{}
	}{
		{IMAGE_VECTOR: images, GRAPH_VECTOR: nodes, ID: ids},
		{IMAGE_VECTOR: lastImage, GRAPH_VECTOR: lastNode, ID: lastIndex},
	} {
		if _, err = stmt.ExecContext(ctx, tC.ID, tC.IMAGE_VECTOR, tC.GRAPH_VECTOR); err != nil {
			t.Errorf("%d/1. (%v): %v", tN, tC.IMAGE_VECTOR, err)
			t.Logf("%d. Vector insert erro %v: ", tN, err)
			continue
		}

		var rows *sql.Rows
		rows, err = conn.QueryContext(ctx,
			"SELECT id, image_vector, graph_vector FROM "+tbl) //nolint:gas
		if err != nil {
			t.Logf("%d. select error erro %v: ", tN, err)
			t.Errorf("%d/3. %v", tN, err)
			continue
		}
		var id interface{}
		var image godror.Vector[float32]
		var node godror.Vector[float32]
		for rows.Next() {
			if err = rows.Scan(&id, &image, &node); err != nil {
				rows.Close()
				t.Errorf("%d/3. scan: %v", tN, err)
				continue
			}

			if err != nil {
				t.Errorf("%d. %v", id, err)
			} else {
				t.Logf("%d. Vector IMAGE_VECTOR read %q: ", id, image)
				t.Logf("%d. Vector GRAPH_VECTOR Sparse read %q: ", id, node)

				v := image.GetValues()
				eq := reflect.DeepEqual(embedding, v)
				if !eq {
					t.Errorf("Got %+v, wanted %+v", v, embedding)
				}
			}
		}
		rows.Close()
	}
}
