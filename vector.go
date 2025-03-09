package godror

/*
#include <stdlib.h>
#include "dpiImpl.h"

// C Wrapper Function to Set the Dimensions Union Field
void setVectorInfoDimensions(dpiVectorInfo *info, void *ptr) {
    info->dimensions.asPtr = ptr;
}

// C Wrapper Function to Get the Dimensions Union Field
void* getVectorInfoDimensions(dpiVectorInfo *info) {
    return info->dimensions.asPtr;
}

*/
import "C"

import (
	"fmt"
	"unsafe"
)

// Format constraint for supported types
type Format interface {
	int8 | float32 | float64 | uint8
}

// Vector holds the embedding VECTOR column starting from 23ai.
type Vector[T Format] struct {
	Indices    []uint32 // Indices of non-zero values (sparse format)
	Dimensions int      // Total dimensions of the vector (can be set explicitly)
	Values     []T      // Non-zero values (sparse format) or all values (dense format)
	IsSparse   bool     // Flag to detect if it's a sparse vector
}

// NewVector creates either a sparse or dense vector
// 1. If indices are provided -> Sparse vector
// 2. If indices are nil -> Dense vector
func NewVector[T Format](values []T, dims int, indices []uint32) Vector[T] {

	// Determine if it's sparse or dense
	if indices != nil {
		return Vector[T]{Indices: indices, Values: values, Dimensions: dims, IsSparse: true}
	}

	// If no indices are provided, it's a dense vector
	// Default dims to length of values and ignored provided dims
	dim := len(values)

	return Vector[T]{Values: values, Dimensions: dim, IsSparse: false}
}

// GetValues returns the values of the vector
func (v Vector[T]) GetValues() []T {
	return v.Values
}

// GetIndices returns the indices of the sparse vector, or nil if dense
func (v Vector[T]) GetIndices() []uint32 {
	if v.IsSparse {
		return v.Indices
	}
	return nil
}

// GetDims returns the dimension of the vector
func (v Vector[T]) GetDims() int {
	return v.Dimensions
}

// IsSparse checks if the vector is sparse
//func (v Vector[T]) IsSparse() bool {
//return v.IsSparse
//}

// String provides a human-readable representation of Vector
func (v Vector[T]) String() string {
	if v.IsSparse {
		// Format: SparseVector(indices: [...], values: [...], dims: X)
		indexStr := fmt.Sprintf("%v", v.GetIndices())
		valueStr := fmt.Sprintf("%v", v.GetValues())
		return fmt.Sprintf("SparseVector(indices: %s, values: %s, dims: %d, format: %T)", indexStr, valueStr, v.Dimensions, *new(T))
	}
	// Format: DenseVector(values: [...], dims: X)
	valueStr := fmt.Sprintf("%v", v.Values)
	//	fmt.Println(" indices ", v.Indices)
	return fmt.Sprintf("DenseVector(values: %s, dims: %d, format: %T)", valueStr, v.Dimensions, *new(T))
}

// GetVectorInfo converts a Go `Vector` into a `dpiVectorInfo` C struct
func GetVectorInfo[T Format](v Vector[T], vectorInfo *C.dpiVectorInfo) error {
	var format C.uint8_t
	var dimensionSize C.uint8_t
	var dataPtr unsafe.Pointer

	switch any(v.Values).(type) {
	case []float32:
		format = C.DPI_VECTOR_FORMAT_FLOAT32
		dimensionSize = 4
		dataPtr = unsafe.Pointer(&v.Values[0])
		//		ptr := (*C.void)(C.malloc(C.size_t(len(v.values)) * C.size_t(unsafe.Sizeof(C.float(0)))))
		//	cArray := (*[1 << 30]C.float)(unsafe.Pointer(ptr))[:len(v.values):len(v.values)]
		//for i, val := range v.values {
		//cArray[i] = C.float(val)
		//	}
		C.setVectorInfoDimensions(vectorInfo, dataPtr)
		if !v.IsSparse {
			v.Dimensions = len(v.Values)
		}

		/*
			if len(v.values) > 0 {
				dimBuffer.asFloat = (*C.float)(unsafe.Pointer(&v.values[0]))
			} else {
				dimBuffer.asFloat = nil
			}
			// Correctly pass Go slice data to C
			dimBuffer.asFloat = (*C.float)(unsafe.Pointer(unsafe.SliceData(v.values))) */

	case []float64:
		format = C.DPI_VECTOR_FORMAT_FLOAT64
		dimensionSize = 8
		//ptr := (*C.double)(C.malloc(C.size_t(len(v.values.([]float64))) * C.size_t(unsafe.Sizeof(C.double(0)))))
		//cArray := (*[1 << 30]C.double)(unsafe.Pointer(ptr))[:len(v.values.([]float64)):len(v.values.([]float64))]
		//for i, val := range v.values.([]float64) {
		//cArray[i] = C.double(val)
		//}
		dataPtr = unsafe.Pointer(&v.Values[0])
	case []int8:
		format = C.DPI_VECTOR_FORMAT_INT8
		dimensionSize = 1
		//ptr := (*C.int8_t)(C.malloc(C.size_t(len(v.values.([]int8))) * C.size_t(unsafe.Sizeof(C.int8_t(0)))))
		//	cArray := (*[1 << 30]C.int8_t)(unsafe.Pointer(ptr))[:len(v.values.([]int8)):len(v.values.([]int8))]
		//for i, v := range v.values.([]int8) {
		//cArray[i] = C.int8_t(v)
		//	}
		dataPtr = unsafe.Pointer(&v.Values[0])
	default:
		panic(fmt.Sprintf("Unsupported type: %T", v.Values))
	}

	// Handle sparse indices
	var sparseIndices *C.uint32_t
	var numSparseValues C.uint32_t
	if v.IsSparse {
		numSparseValues = C.uint32_t(len(v.Indices))
		ptr := (*C.uint32_t)(C.malloc(C.size_t(numSparseValues) * C.size_t(unsafe.Sizeof(C.uint32_t(0)))))
		cArray := (*[1 << 30]C.uint32_t)(unsafe.Pointer(ptr))[:numSparseValues:numSparseValues]
		for i, val := range v.Indices {
			cArray[i] = C.uint32_t(val)
		}
		sparseIndices = ptr
	} else {
		numSparseValues = 0
		sparseIndices = nil
	}

	vectorInfo.format = format
	vectorInfo.numDimensions = C.uint32_t(v.Dimensions)
	vectorInfo.dimensionSize = C.uint8_t(dimensionSize)
	vectorInfo.numSparseValues = C.uint32_t(numSparseValues)
	vectorInfo.sparseIndices = sparseIndices
	return nil
}

// SetVectorInfo converts a C `dpiVectorInfo` struct into a Go `Vector`
func SetVectorInfo[T Format](vecInfo *C.dpiVectorInfo) Vector[T] {
	var values []T
	var indices []uint32
	var isSparse bool

	// Determine data format
	switch vecInfo.format {
	case C.DPI_VECTOR_FORMAT_FLOAT32: // float32
		var nonZeroVal = vecInfo.numDimensions
		if vecInfo.numSparseValues > 0 {
			isSparse = true
			indices = make([]uint32, vecInfo.numSparseValues)
			ptr := (*[1 << 30]C.uint32_t)(unsafe.Pointer(vecInfo.sparseIndices))[:vecInfo.numSparseValues:vecInfo.numSparseValues]
			for i, v := range ptr {
				indices[i] = uint32(v)
			}
			nonZeroVal = vecInfo.numSparseValues
			values = make([]T, vecInfo.numSparseValues)
		} else {
			values = make([]T, vecInfo.numDimensions)
		}
		ptr := (*[1 << 30]float32)(unsafe.Pointer(C.getVectorInfoDimensions(vecInfo)))[:nonZeroVal:nonZeroVal]
		for i, v := range ptr {
			values[i] = T(v) // Convert C float to Go T
		}
		//copy(values.([]float32), ptr)
	case C.DPI_VECTOR_FORMAT_FLOAT64: // float64
		values = make([]T, vecInfo.numDimensions)
		ptr := (*[1 << 30]float64)(unsafe.Pointer(C.getVectorInfoDimensions(vecInfo)))[:vecInfo.numDimensions:vecInfo.numDimensions]
		for i, v := range ptr {
			values[i] = T(v) // Convert C float to Go T
		}
		//copy(values.([]float64), ptr)
	case C.DPI_VECTOR_FORMAT_INT8: // int8
		values = make([]T, vecInfo.numDimensions)
		ptr := (*[1 << 30]int8)(unsafe.Pointer(C.getVectorInfoDimensions(vecInfo)))[:vecInfo.numDimensions:vecInfo.numDimensions]
		for i, v := range ptr {
			values[i] = T(v) // Convert C float to Go T
		}
		//copy(values.([]int8), ptr)
	default:
		panic(fmt.Sprintf("Unknown format: %d", vecInfo.format))
	}

	return Vector[T]{
		Indices:    indices,
		Dimensions: int(vecInfo.numDimensions),
		Values:     values,
		IsSparse:   isSparse,
	}
}
