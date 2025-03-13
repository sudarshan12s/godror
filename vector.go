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

// Storage Format supported types
type Format interface {
	int8 | float32 | float64 | uint8
}

// Vector holds the embedding VECTOR column starting from 23ai.
type Vector[T Format] struct {
	Indices    []uint32 // Indices of non-zero values (sparse format)
	Dimensions uint32   // Total dimensions of the vector (can be set explicitly)
	Values     []T      // Non-zero values (sparse format) or all values (dense format)
	IsSparse   bool     // Flag to detect if it's a sparse vector
}

// String provides a human-readable representation of Vector
func (v Vector[T]) String() string {
	if v.IsSparse {
		// Format: SparseVector(indices: [...], values: [...], dims: X)
		indexStr := fmt.Sprintf("%v", v.Indices)
		valueStr := fmt.Sprintf("%v", v.Values)
		return fmt.Sprintf("SparseVector(indices: %s, values: %s, dims: %d, format: %T)", indexStr, valueStr, v.Dimensions, *new(T))
	}
	// Format: DenseVector(values: [...], dims: X)
	valueStr := fmt.Sprintf("%v", v.Values)
	return fmt.Sprintf("DenseVector(values: %s, dims: %d, format: %T)", valueStr, v.Dimensions, *new(T))
}

// GetVectorInfo converts a Go `Vector` into a `dpiVectorInfo` C struct
func GetVectorInfo[T Format](v Vector[T], vectorInfo *C.dpiVectorInfo) error {
	var format C.uint8_t
	var dimensionSize C.uint8_t
	var numDims C.uint32_t
	var dataPtr unsafe.Pointer

	numDims = C.uint32_t(len(v.Values))
	switch any(v.Values).(type) {
	case []float32:
		format = C.DPI_VECTOR_FORMAT_FLOAT32
		dimensionSize = 4
		if numDims > 0 {
			dataPtr = unsafe.Pointer(&v.Values[0])
		} else {
			dataPtr = unsafe.Pointer(nil)
		}
	case []float64:
		format = C.DPI_VECTOR_FORMAT_FLOAT64
		dimensionSize = 8
		if numDims > 0 {
			dataPtr = unsafe.Pointer(&v.Values[0])
		} else {
			dataPtr = unsafe.Pointer(nil)
		}
	case []int8:
		format = C.DPI_VECTOR_FORMAT_INT8
		dimensionSize = 1
		if numDims > 0 {
			ptr := (*C.int8_t)(C.malloc(C.size_t(numDims) * C.size_t(unsafe.Sizeof(C.int8_t(0)))))
			cArray := (*[1 << 30]C.int8_t)(unsafe.Pointer(ptr))[:numDims:numDims]
			for i, v := range v.Values {
				cArray[i] = C.int8_t(v)
			}
			dataPtr = unsafe.Pointer(ptr)
		} else {
			dataPtr = unsafe.Pointer(nil)
		}
	case []uint8:
		format = C.DPI_VECTOR_FORMAT_BINARY
		dimensionSize = 1
		if numDims > 0 {
			ptr := (*C.uint8_t)(C.malloc(C.size_t(numDims) * C.size_t(unsafe.Sizeof(C.uint8_t(0)))))
			cArray := (*[1 << 30]C.uint8_t)(unsafe.Pointer(ptr))[:numDims:numDims]
			for i, v := range v.Values {
				cArray[i] = C.uint8_t(v)
			}
			dataPtr = unsafe.Pointer(ptr)
		} else {
			dataPtr = unsafe.Pointer(nil)
		}
	default:
		panic(fmt.Sprintf("Unsupported type: %T", v.Values))
	}
	if !v.IsSparse {
		var multiplier uint32 = 1
		if format == C.DPI_VECTOR_FORMAT_BINARY {
			// Binary vector is not supported for Sparse
			multiplier = 8
		}
		v.Dimensions = multiplier * uint32(len(v.Values)) // avoid updating this user ptr v
	}
	C.setVectorInfoDimensions(vectorInfo, dataPtr)

	// Handle sparse indices
	var sparseIndices *C.uint32_t
	//	var sparseIndices unsafe.Pointer
	var numSparseValues C.uint32_t
	if v.IsSparse {
		if len(v.Indices) > 0 {
			numSparseValues = C.uint32_t(len(v.Indices))
			//	sparseIndices = (*C.uint32_t)(unsafe.Pointer(&v.Indices[0]))
			ptr := (*C.uint32_t)(C.malloc(C.size_t(numSparseValues) * C.size_t(unsafe.Sizeof(C.uint32_t(0)))))
			cArray := (*[1 << 30]C.uint32_t)(unsafe.Pointer(ptr))[:numSparseValues:numSparseValues]
			for i, val := range v.Indices {
				cArray[i] = C.uint32_t(val)
			}
			sparseIndices = ptr
		}
	} else {
		numSparseValues = 0
		sparseIndices = (*C.uint32_t)(unsafe.Pointer(nil))
	}

	vectorInfo.format = format
	vectorInfo.numDimensions = C.uint32_t(v.Dimensions)
	vectorInfo.dimensionSize = C.uint8_t(dimensionSize)
	vectorInfo.numSparseValues = C.uint32_t(numSparseValues)
	vectorInfo.sparseIndices = (*C.uint32_t)(sparseIndices)
	return nil
}

// Go wrapper function
func GetVectorInfoDimensions(info *C.dpiVectorInfo) unsafe.Pointer {
	return C.getVectorInfoDimensions(info)
}

// SetVectorInfo converts a C `dpiVectorInfo` struct into a Go `Vector`
func SetVectorInfo[T Format](vecInfo *C.dpiVectorInfo) Vector[T] {
	var values []T
	var indices []uint32
	var isSparse bool

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
	// Determine data format
	switch vecInfo.format {
	case C.DPI_VECTOR_FORMAT_FLOAT32: // float32
		ptr := (*[1 << 30]float32)(unsafe.Pointer(C.getVectorInfoDimensions(vecInfo)))[:nonZeroVal:nonZeroVal]
		for i, v := range ptr {
			values[i] = T(v) // Convert C type to Go T
		}
	case C.DPI_VECTOR_FORMAT_FLOAT64: // float64
		ptr := (*[1 << 30]float64)(unsafe.Pointer(C.getVectorInfoDimensions(vecInfo)))[:nonZeroVal:nonZeroVal]
		for i, v := range ptr {
			values[i] = T(v) // Convert C type to Go T
		}
	case C.DPI_VECTOR_FORMAT_INT8: // int8
		ptr := (*[1 << 30]int8)(unsafe.Pointer(C.getVectorInfoDimensions(vecInfo)))[:nonZeroVal:nonZeroVal]
		for i, v := range ptr {
			values[i] = T(v) // Convert C type to Go T
		}
	case C.DPI_VECTOR_FORMAT_BINARY: // uint8
		actualSize := (vecInfo.numDimensions) / 8
		values = make([]T, actualSize)
		ptr := (*[1 << 30]uint8)(unsafe.Pointer(C.getVectorInfoDimensions(vecInfo)))[:actualSize:actualSize]
		for i, v := range ptr {
			values[i] = T(v) // Convert C type to Go T
		}
	default:
		panic(fmt.Sprintf("Unknown format: %d", vecInfo.format))
	}

	return Vector[T]{
		Indices:    indices,
		Dimensions: uint32(vecInfo.numDimensions),
		Values:     values,
		IsSparse:   isSparse,
	}
}
