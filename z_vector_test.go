// Copyright 2025 The Godror Authors
//
//
// SPDX-License-Identifier: UPL-1.0 OR Apache-2.0

package godror_test

import (
	"context"
	"database/sql"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	godror "github.com/godror/godror"
)

func compareDenseVector(t *testing.T, id godror.Number, got godror.Vector, expected godror.Vector) {
	t.Helper()
	if !reflect.DeepEqual(got.Values, expected.Values) {
		t.Errorf("ID %v: expected %v, got %v", id, expected.Values, got.Values)
	}
}

func compareSparseVector(t *testing.T, id godror.Number, got godror.Vector, expected godror.Vector) {
	t.Helper()
	if !reflect.DeepEqual(got.Values, expected.Values) {
		t.Errorf("ID %s: Sparse godror.Vector values mismatch. Got %+v, expected %+v", id, got.Values, expected.Values)
	}
	if !reflect.DeepEqual(got.Indices, expected.Indices) {
		t.Errorf("ID %s: Sparse godror.Vector indices mismatch. Got %+v, expected %+v", id, got.Indices, expected.Indices)
	}
	if got.Dimensions != expected.Dimensions {
		t.Errorf("ID %s: Sparse godror.Vector dimensions mismatch. Got %d, expected %d", id, got.Dimensions, expected.Dimensions)
	}
}

// It Verifies returning godror.Vector columns in outbinds
func TestVectorOutBinds(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(testContext("OutBindsVector"), 30*time.Second)
	defer cancel()
	conn, err := testDb.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	tbl := "test_vector_outbind" + tblSuffix
	conn.ExecContext(ctx, "DROP TABLE "+tbl)
	_, err = conn.ExecContext(ctx,
		`CREATE TABLE `+tbl+` (
			id NUMBER(6), 
			image_vector Vector, 
			graph_vector Vector(5, float32, SPARSE), 
			int_vector Vector(3, int8), 
			float_vector Vector(4, float64), 
			sparse_int_vector Vector(4, int8, SPARSE)
		)`,
	)
	if err != nil {
		if errIs(err, 902, "invalid datatype") {
			t.Skip(err)
		}
		t.Fatal(err)
	}
	t.Logf("Vector table %q created", tbl)
	defer testDb.Exec("DROP TABLE " + tbl)

	stmt, err := conn.PrepareContext(ctx,
		`INSERT INTO `+tbl+` (id, image_vector, graph_vector, int_vector, float_vector, sparse_int_vector) 
		 VALUES (:1, :2, :3, :4, :5, :6) RETURNING image_vector, graph_vector, int_vector, float_vector, sparse_int_vector INTO :7, :8, :9, :10, :11`,
	)
	if err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()

	// Test values
	var embedding = []float32{1.1, 2.2, 3.3}
	var sparseValuesF32 = []float32{0.5, 1.2, -0.9}
	var sparseIndicesF32 = []uint32{0, 2, 3}
	var sparseDimensionsF32 uint32 = 5

	var intValues = []int8{1, -5, 3}
	var floatValues = []float64{10.5, 20.3, -5.5, 3.14}

	var sparseValuesI8 = []int8{-1, 4, -7}
	var sparseIndicesI8 = []uint32{1, 2, 3}
	var sparseDimensionsI8 uint32 = 4

	var id = godror.Number("1")

	// Out bind variables
	var outImage godror.Vector
	var outGraph godror.Vector
	var outInt godror.Vector
	var outFloat godror.Vector
	var outSparseInt godror.Vector

	_, err = stmt.ExecContext(ctx,
		1, // ID
		godror.Vector{Values: embedding},
		godror.Vector{Values: sparseValuesF32, Indices: sparseIndicesF32, Dimensions: sparseDimensionsF32, IsSparse: true},
		godror.Vector{Values: intValues},
		godror.Vector{Values: floatValues},
		godror.Vector{Values: sparseValuesI8, Indices: sparseIndicesI8, Dimensions: sparseDimensionsI8, IsSparse: true},
		sql.Out{Dest: &outImage}, sql.Out{Dest: &outGraph}, sql.Out{Dest: &outInt}, sql.Out{Dest: &outFloat}, sql.Out{Dest: &outSparseInt},
	)
	if err != nil {
		t.Fatalf("ExecContext failed: %v", err)
	}

	// Validate out bind values
	compareDenseVector(t, id, outImage, godror.Vector{Values: embedding})
	compareDenseVector(t, id, outInt, godror.Vector{Values: intValues})
	compareDenseVector(t, id, outFloat, godror.Vector{Values: floatValues})

	compareSparseVector(t, id, outGraph, godror.Vector{Values: sparseValuesF32, Indices: sparseIndicesF32, Dimensions: sparseDimensionsF32, IsSparse: true})
	compareSparseVector(t, id, outSparseInt, godror.Vector{Values: sparseValuesI8, Indices: sparseIndicesI8, Dimensions: sparseDimensionsI8, IsSparse: true})
}

// It Verifies insert and read of godror.Vector columns.
// empty values are also given to see an error ORA-21560 is reported.
func TestVectorReadWrite(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(testContext("ReadWriteVector"), 30*time.Second)
	defer cancel()
	conn, err := testDb.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	tbl := "test_vector_table" + tblSuffix
	conn.ExecContext(ctx, "DROP TABLE "+tbl)
	_, err = conn.ExecContext(ctx,
		`CREATE TABLE `+tbl+` (
			id NUMBER(6), 
			image_vector Vector, 
			graph_vector Vector(5, float32, SPARSE), 
			float64_sparse Vector(6, float64, SPARSE), 
			int_vector Vector(3, int8), 
			float_vector Vector(4, float64), 
			uint_vector Vector(16, binary)
		)`,
	)
	if err != nil {
		if errIs(err, 902, "invalid datatype") {
			t.Skip(err)
		}
		t.Fatal(err)
	}
	t.Logf("Vector table %q created", tbl)
	defer testDb.Exec("DROP TABLE " + tbl)

	stmt, err := conn.PrepareContext(ctx,
		`INSERT INTO `+tbl+` (id, image_vector, graph_vector, float64_sparse, int_vector, float_vector, uint_vector) VALUES (:1, :2, :3, :4, :5, :6, :7)`,
	)
	if err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()

	// Test values for each type
	var embedding = []float32{1.1, 2.2, 3.3}
	var sparseValues = []float32{0.5, 1.2, -0.9}
	var sparseIndices = []uint32{0, 2, 3}
	var sparseDimensions uint32 = 5

	var sparseFloat64Values = []float64{2.5, -1.1, 4.7}
	var sparseFloat64Indices = []uint32{1, 3, 5}
	var sparseFloat64Dimensions uint32 = 6

	var emptyValues = []float32{}
	var emptyIndices = []uint32{}

	var emptyFloat64Values = []float64{}
	var emptyFloat64Indices = []uint32{}

	var intValues = []int8{1, -5, 3}
	var emptyIntValues = []int8{}

	var floatValues = []float64{10.5, 20.3, -5.5, 3.14}
	var emptyFloatValues = []float64{}

	var uintValues = []uint8{255, 100}
	var emptyUintValues = []uint8{}

	// Define test cases
	testCases := []struct {
		ID            godror.Number
		ImageVector   godror.Vector
		GraphVector   godror.Vector
		Float64Sparse godror.Vector
		IntVector     godror.Vector
		FloatVector   godror.Vector
		UintVector    godror.Vector
	}{
		// Normal values
		{"1", godror.Vector{Values: embedding},
			godror.Vector{Values: sparseValues, Indices: sparseIndices, Dimensions: sparseDimensions, IsSparse: true},
			godror.Vector{Values: sparseFloat64Values, Indices: sparseFloat64Indices, Dimensions: sparseFloat64Dimensions, IsSparse: true},
			godror.Vector{Values: intValues},
			godror.Vector{Values: floatValues},
			godror.Vector{Values: uintValues}},
		{"2", godror.Vector{Values: emptyValues},
			godror.Vector{Values: sparseValues, Indices: sparseIndices, Dimensions: sparseDimensions, IsSparse: true},
			godror.Vector{Values: sparseFloat64Values, Indices: sparseFloat64Indices, Dimensions: sparseFloat64Dimensions, IsSparse: true},
			godror.Vector{Values: intValues},
			godror.Vector{Values: floatValues},
			godror.Vector{Values: uintValues}},
		{"3", godror.Vector{Values: embedding},
			godror.Vector{Values: emptyValues, Indices: emptyIndices, Dimensions: sparseDimensions, IsSparse: true},
			godror.Vector{Values: sparseFloat64Values, Indices: sparseFloat64Indices, Dimensions: sparseFloat64Dimensions, IsSparse: true},
			godror.Vector{Values: intValues},
			godror.Vector{Values: floatValues},
			godror.Vector{Values: uintValues}},
		{"4", godror.Vector{Values: embedding},
			godror.Vector{Values: sparseValues, Indices: sparseIndices, Dimensions: sparseFloat64Dimensions, IsSparse: true},
			godror.Vector{Values: emptyFloat64Values, Indices: emptyFloat64Indices, Dimensions: sparseFloat64Dimensions, IsSparse: true},
			godror.Vector{Values: intValues},
			godror.Vector{Values: floatValues},
			godror.Vector{Values: uintValues}},
		{"5", godror.Vector{Values: embedding},
			godror.Vector{Values: sparseValues, Indices: sparseIndices, Dimensions: sparseFloat64Dimensions, IsSparse: true},
			godror.Vector{Values: sparseFloat64Values, Indices: sparseFloat64Indices, Dimensions: sparseFloat64Dimensions, IsSparse: true},
			godror.Vector{Values: emptyIntValues},
			godror.Vector{Values: floatValues},
			godror.Vector{Values: uintValues}},
		{"6", godror.Vector{Values: embedding},
			godror.Vector{Values: sparseValues, Indices: sparseIndices, Dimensions: sparseFloat64Dimensions, IsSparse: true},
			godror.Vector{Values: sparseFloat64Values, Indices: sparseFloat64Indices, Dimensions: sparseFloat64Dimensions, IsSparse: true},
			godror.Vector{Values: intValues},
			godror.Vector{Values: emptyFloatValues},
			godror.Vector{Values: uintValues}},
		{"7", godror.Vector{Values: embedding},
			godror.Vector{Values: sparseValues, Indices: sparseIndices, Dimensions: sparseFloat64Dimensions, IsSparse: true},
			godror.Vector{Values: sparseFloat64Values, Indices: sparseFloat64Indices, Dimensions: sparseFloat64Dimensions, IsSparse: true},
			godror.Vector{Values: intValues},
			godror.Vector{Values: floatValues},
			godror.Vector{Values: emptyUintValues}},
	}

	// Insert and validate each test case
	for _, tC := range testCases {
		if _, err = stmt.ExecContext(ctx, tC.ID, tC.ImageVector, tC.GraphVector, tC.Float64Sparse, tC.IntVector, tC.FloatVector, tC.UintVector); err != nil {
			if tC.ID != "1" && strings.Contains(err.Error(), "ORA-21560") { // empty godror.Vector
				// Expected Error ORA-21560
				continue
			}
			t.Errorf("Insert failed for ID %s: %v", tC.ID, err)
			continue
		}

		var gotImage godror.Vector
		var gotGraph godror.Vector
		var gotFloat64Sparse godror.Vector
		var gotInt godror.Vector
		var gotFloat godror.Vector
		var gotUint godror.Vector

		row := conn.QueryRowContext(ctx, `SELECT image_vector, graph_vector, float64_sparse, int_vector, float_vector, uint_vector FROM `+tbl+` WHERE id = :1`, tC.ID)
		err = row.Scan(&gotImage, &gotGraph, &gotFloat64Sparse, &gotInt, &gotFloat, &gotUint)
		if err != nil {
			t.Errorf("Select failed for ID %s: %v", tC.ID, err)
			continue
		}

		// Compare all godror.Vector types
		compareDenseVector(t, tC.ID, gotImage, tC.ImageVector)
		compareSparseVector(t, tC.ID, gotGraph, tC.GraphVector)
		compareSparseVector(t, tC.ID, gotFloat64Sparse, tC.Float64Sparse) // float64 sparse comparison
		compareDenseVector(t, tC.ID, gotInt, tC.IntVector)
		compareDenseVector(t, tC.ID, gotFloat, tC.FloatVector)
		compareDenseVector(t, tC.ID, gotUint, tC.UintVector) // uint8 remains dense
	}
}

// It Verifies batch insert of Vector columns and verify the inserted rows.
func TestVectorReadWriteBatch(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(testContext("ReadWriteVector"), 30*time.Second)
	defer cancel()
	conn, err := testDb.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	tbl := "test_vector_batch" + tblSuffix
	conn.ExecContext(ctx, "DROP TABLE "+tbl)
	_, err = conn.ExecContext(ctx,
		"CREATE TABLE "+tbl+" (id NUMBER(6), image_vector Vector, graph_vector Vector(5, float32, SPARSE) )", //nolint:gas
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
	var sparseValues = []float32{0.5, 1.2, -0.9}
	var sparseIndices = []uint32{0, 2, 3}
	var sparseDimensions uint32 = 5

	graph_vector := godror.Vector{
		Indices:    sparseIndices,
		Dimensions: sparseDimensions,
		Values:     sparseValues,
		IsSparse:   true,
	}
	image_vector := godror.Vector{Values: embedding}

	// values for batch insert
	const num = 10 // batch size
	ids := make([]godror.Number, num)
	images := make([]godror.Vector, num)
	nodes := make([]godror.Vector, num)
	for i := range ids {
		nodes[i] = graph_vector
		images[i] = image_vector
		ids[i] = godror.Number(strconv.Itoa(i))
	}

	// value for last row to simulate single row insert
	lastIndex := godror.Number(strconv.Itoa(num))
	lastImage := image_vector
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
			t.Logf("%d. godror.Vector insert error %v: ", tN, err)
			continue
		}

		var rows *sql.Rows
		rows, err = conn.QueryContext(ctx,
			"SELECT id, image_vector, graph_vector FROM "+tbl) //nolint:gas
		if err != nil {
			t.Logf("%d. select error error %v: ", tN, err)
			t.Errorf("%d/3. %v", tN, err)
			continue
		}

		var index godror.Number // Default to 0 for batch insert case.
		if tN == 0 {
			if ids, ok := tC.ID.([]godror.Number); ok && len(ids) > 0 {
				index = ids[0]
			}
		} else {
			if id, ok := tC.ID.(godror.Number); ok {
				index = id
			}
		}

		var id interface{}
		var image godror.Vector
		var node godror.Vector
		for rows.Next() {
			if err = rows.Scan(&id, &image, &node); err != nil {
				rows.Close()
				t.Errorf("%d/3. scan: %v", tN, err)
				continue
			}
			if err != nil {
				t.Errorf("%d. %v", id, err)
			} else {
				t.Logf("%d. godror.Vector IMAGE_VECTOR read %q: ", id, image)
				t.Logf("%d. godror.Vector GRAPH_VECTOR Sparse read %q: ", id, node)
				compareDenseVector(t, index, image, image_vector)
				compareSparseVector(t, index, node, graph_vector)
			}
		}
		rows.Close()
	}
}

// It Verifies Flex storage godror.Vector columns.
func TestVectorFlex(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(testContext("BindsFlexVector"), 30*time.Second)
	defer cancel()

	conn, err := testDb.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	tbl := "test_vector_flexbind" + tblSuffix
	conn.ExecContext(ctx, "DROP TABLE "+tbl)
	_, err = conn.ExecContext(ctx,
		`CREATE TABLE `+tbl+` (
			id NUMBER(6), 
			image_vector Vector(*,*),
			graph_vector Vector(*, *, SPARSE), 
			int_vector Vector(*, *), 
			float_vector Vector(*, *), 
			sparse_int_vector Vector(*, *, SPARSE)
		)`,
	)
	if err != nil {
		if errIs(err, 902, "invalid datatype") {
			t.Skip(err)
		}
		t.Fatal(err)
	}
	t.Logf("Vector table %q created", tbl)
	defer testDb.Exec("DROP TABLE " + tbl)

	stmt, err := conn.PrepareContext(ctx,
		`INSERT INTO `+tbl+` (id, image_vector, graph_vector, int_vector, float_vector, sparse_int_vector) 
		 VALUES (:1, :2, :3, :4, :5, :6) RETURNING image_vector, graph_vector, int_vector, float_vector, sparse_int_vector INTO :7, :8, :9, :10, :11`,
	)
	if err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()

	// Test values
	var embedding = []float32{1.1, 2.2, 3.3}
	var sparseValuesF32 = []float32{0.5, 1.2, -0.9}
	var sparseIndicesF32 = []uint32{0, 2, 3}
	var sparseDimensionsF32 uint32 = 5

	var intValues = []int8{1, -5, 3}
	var floatValues = []float64{10.5, 20.3, -5.5, 3.14}

	var sparseValuesI8 = []int8{-1, 4, -7}
	var sparseIndicesI8 = []uint32{1, 2, 3}
	var sparseDimensionsI8 uint32 = 4

	var id = godror.Number("1")

	// Out bind variables
	var outImage godror.Vector
	var outGraph godror.Vector
	var outInt godror.Vector
	var outFloat godror.Vector
	var outSparseInt godror.Vector

	_, err = stmt.ExecContext(ctx,
		1, // ID
		godror.Vector{Values: embedding},
		godror.Vector{Values: sparseValuesF32, Indices: sparseIndicesF32, Dimensions: sparseDimensionsF32, IsSparse: true},
		godror.Vector{Values: intValues},
		godror.Vector{Values: floatValues},
		godror.Vector{Values: sparseValuesI8, Indices: sparseIndicesI8, Dimensions: sparseDimensionsI8, IsSparse: true},
		sql.Out{Dest: &outImage}, sql.Out{Dest: &outGraph}, sql.Out{Dest: &outInt}, sql.Out{Dest: &outFloat}, sql.Out{Dest: &outSparseInt},
	)
	if err != nil {
		t.Fatalf("ExecContext failed: %v", err)
	}

	// Validate out bind values
	compareDenseVector(t, id, outImage, godror.Vector{Values: embedding})
	compareDenseVector(t, id, outInt, godror.Vector{Values: intValues})
	compareDenseVector(t, id, outFloat, godror.Vector{Values: floatValues})

	compareSparseVector(t, id, outGraph, godror.Vector{Values: sparseValuesF32, Indices: sparseIndicesF32, Dimensions: sparseDimensionsF32, IsSparse: true})
	compareSparseVector(t, id, outSparseInt, godror.Vector{Values: sparseValuesI8, Indices: sparseIndicesI8, Dimensions: sparseDimensionsI8, IsSparse: true})

	row := conn.QueryRowContext(ctx, `SELECT image_vector, graph_vector, int_vector, float_vector, sparse_int_vector FROM `+tbl+` WHERE id = 1`)
	err = row.Scan(&outImage, &outGraph, &outInt, &outFloat, &outSparseInt)
	if err != nil {
		t.Errorf("Select failed for ID 1: %v", err)
	}

	// Compare read values
	compareDenseVector(t, id, outImage, godror.Vector{Values: embedding})
	compareDenseVector(t, id, outInt, godror.Vector{Values: intValues})
	compareDenseVector(t, id, outFloat, godror.Vector{Values: floatValues})

	compareSparseVector(t, id, outGraph, godror.Vector{Values: sparseValuesF32, Indices: sparseIndicesF32, Dimensions: sparseDimensionsF32, IsSparse: true})
	compareSparseVector(t, id, outSparseInt, godror.Vector{Values: sparseValuesI8, Indices: sparseIndicesI8, Dimensions: sparseDimensionsI8, IsSparse: true})
}
