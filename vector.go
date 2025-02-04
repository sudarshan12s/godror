package godror

import "C"

import (
	"fmt"
	"reflect"
	"unsafe"
)

// Vector holds the embedding VECTOR column starting from 23ai.
type Vector[T int8 | float32 | float64 | uint8] struct {
	indices    []uint32    // Indices of non-zero values (sparse format)
	dimensions int         // Total dimensions of the vector (can be set explicitly)
	values     interface{} // Non-zero values (sparse format) or all values (dense format)
	isSparse   bool        // Flag to detect if it's a sparse vector
	typ        string      // Store the type as a string for display
}

// NewVector creates either a sparse or dense vector
// 1. If indices are provided -> Sparse vector
// 2. If indices are nil -> Dense vector
func NewVector(values interface{}, indices []int, dims ...int) Vector {
	// Detect the type of values
	var typ string
	switch reflect.TypeOf(values).Kind() {
	case reflect.Slice:
		elemType := reflect.TypeOf(values).Elem().Kind()
		switch elemType {
		case reflect.Float32:
			typ = "float32"
		case reflect.Float64:
			typ = "float64"
		case reflect.Int8:
			typ = "int8"
		default:
			typ = fmt.Sprintf("unknown: %v", elemType)
		}
	default:
		typ = "not a slice"
	}

	// Determine if it's sparse or dense
	if indices != nil {
		return Vector{indices: indices, values: values, dims: dims[0], isSparse: true, typ: typ}
	}

	// If no indices are provided, it's a dense vector
	// Default dims to length of values if not provided
	dim := len(values.([]interface{}))
	if len(dims) > 0 {
		dim = dims[0] // Use user-defined dimension if provided
	}

	return Vector{values: values, dimensions: dim, isSparse: false, typ: typ}
}

// GetValues returns the values of the vector
func (v Vector) GetValues() interface{} {
	return v.values
}

// GetIndices returns the indices of the sparse vector, or nil if dense
func (v Vector) GetIndices() []uint32 {
	if v.isSparse {
		return v.indices
	}
	return nil
}

// GetDims returns the dimension of the vector
func (v Vector) GetDims() int {
	return v.dimensions
}

// IsSparse checks if the vector is sparse
func (v Vector) IsSparse() bool {
	return v.isSparse
}

// String provides a human-readable representation of Vector
func (v Vector) String() string {
	if v.isSparse {
		// Format: SparseVector(indices: [...], values: [...], dims: X)
		indexStr := fmt.Sprintf("%v", v.indices)
		valueStr := fmt.Sprintf("%v", v.values)
		return fmt.Sprintf("SparseVector(indices: %s, values: %s, dims: %d, type: %s)", indexStr, valueStr, v.dims, v.typ)
	}
	// Format: DenseVector(values: [...], dims: X)
	valueStr := fmt.Sprintf("%v", v.values)
	return fmt.Sprintf("DenseVector(values: %s, dims: %d, type: %s)", valueStr, v.dims, v.typ)
}

// GetVectorInfo converts a Go `Vector` into a `dpiVectorInfo` C struct
func GetVectorInfo(hv Vector) C.dpiVectorInfo {
	var format C.uint8_t
	var dimBuffer C.dpiVectorDimensionBuffer
	var dimensionSize C.uint8_t

	switch hv.typ {
	case "float32":
		format = C.DPI_VECTOR_FORMAT_FLOAT32
		dimensionSize = 4
		ptr := (*C.float)(C.malloc(C.size_t(len(hv.values.([]float32))) * C.size_t(unsafe.Sizeof(C.float(0)))))
		cArray := (*[1 << 30]C.float)(unsafe.Pointer(ptr))[:len(hv.values.([]float32)):len(hv.values.([]float32))]
		for i, v := range hv.values.([]float32) {
			cArray[i] = C.float(v)
		}
		dimBuffer.asFloat = ptr
	case "float64":
		format = C.DPI_VECTOR_FORMAT_FLOAT64
		dimensionSize = 8
		ptr := (*C.double)(C.malloc(C.size_t(len(hv.values.([]float64))) * C.size_t(unsafe.Sizeof(C.double(0)))))
		cArray := (*[1 << 30]C.double)(unsafe.Pointer(ptr))[:len(hv.values.([]float64)):len(hv.values.([]float64))]
		for i, v := range hv.values.([]float64) {
			cArray[i] = C.double(v)
		}
		dimBuffer.asDouble = ptr
	case "int8":
		format = DPI_VECTOR_FORMAT_INT8
		dimensionSize = 1
		ptr := (*C.int8_t)(C.malloc(C.size_t(len(hv.values.([]int8))) * C.size_t(unsafe.Sizeof(C.int8_t(0)))))
		cArray := (*[1 << 30]C.int8_t)(unsafe.Pointer(ptr))[:len(hv.values.([]int8)):len(hv.values.([]int8))]
		for i, v := range hv.values.([]int8) {
			cArray[i] = C.int8_t(v)
		}
		dimBuffer.asInt8 = ptr
	default:
		panic(fmt.Sprintf("Unsupported type: %s", hv.typ))
	}

	// Handle sparse indices
	var sparseIndices *C.uint32_t
	var numSparseValues C.uint32_t
	if hv.isSparse {
		numSparseValues = C.uint32_t(len(hv.indices))
		ptr := (*C.uint32_t)(C.malloc(C.size_t(numSparseValues) * C.size_t(unsafe.Sizeof(C.uint32_t(0)))))
		cArray := (*[1 << 30]C.uint32_t)(unsafe.Pointer(ptr))[:numSparseValues:numSparseValues]
		for i, v := range hv.indices {
			cArray[i] = C.uint32_t(v)
		}
		sparseIndices = ptr
	} else {
		numSparseValues = 0
		sparseIndices = nil
	}

	return C.dpiVectorInfo{
		format:          format,
		numDimensions:   C.uint32_t(hv.dims),
		dimensionSize:   dimensionSize,
		dimensions:      dimBuffer,
		numSparseValues: numSparseValues,
		sparseIndices:   sparseIndices,
	}
}

// SetVectorInfo converts a C `dpiVectorInfo` struct into a Go `Vector`
func SetVectorInfo(vecInfo C.dpiVectorInfo) Vector {
	var values interface{}
	var indices []int
	var isSparse bool
	var typ string

	// Determine data format
	switch vecInfo.format {
	case C.DPI_VECTOR_FORMAT_FLOAT32: // float32
		typ = "float32"
		values = make([]float32, vecInfo.numDimensions)
		ptr := (*[1 << 30]float32)(unsafe.Pointer(vecInfo.dimensions.asPtr))[:vecInfo.numDimensions:vecInfo.numDimensions]
		copy(values.([]float32), ptr)
	case C.DPI_VECTOR_FORMAT_FLOAT64: // float64
		typ = "float64"
		values = make([]float64, vecInfo.numDimensions)
		ptr := (*[1 << 30]float64)(unsafe.Pointer(vecInfo.dimensions.asPtr))[:vecInfo.numDimensions:vecInfo.numDimensions]
		copy(values.([]float64), ptr)
	case C.DPI_VECTOR_FORMAT_INT8: // int8
		typ = "int8"
		values = make([]int8, vecInfo.numDimensions)
		ptr := (*[1 << 30]int8)(unsafe.Pointer(vecInfo.dimensions.asPtr))[:vecInfo.numDimensions:vecInfo.numDimensions]
		copy(values.([]int8), ptr)
	default:
		panic(fmt.Sprintf("Unknown format: %d", vecInfo.format))
	}

	// Handle sparse case
	if vecInfo.numSparseValues > 0 {
		isSparse = true
		indices = make([]int, vecInfo.numSparseValues)
		ptr := (*[1 << 30]C.uint32_t)(unsafe.Pointer(vecInfo.sparseIndices))[:vecInfo.numSparseValues:vecInfo.numSparseValues]
		for i, v := range ptr {
			indices[i] = int(v)
		}
	}

	return Vector{
		indices:  indices,
		dims:     int(vecInfo.numDimensions),
		values:   values,
		isSparse: isSparse,
		typ:      typ,
	}
}
