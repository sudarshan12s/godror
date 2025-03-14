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

// Vector holds the embedding VECTOR column starting from 23ai.
type Vector struct {
	Indices    []uint32    // Indices of non-zero values (sparse format).
	Dimensions uint32      // Total dimensions of the vector.
	Values     interface{} // Non-zero values (sparse format) or all values (dense format).
	IsSparse   bool        // Flag to detect if it's a sparse vector
}

// It generates in below format:
// For SPARSE: [dim, [index], [values]]
// For Dense: [values]
func (v Vector) String() string {
	if v.IsSparse {
		return fmt.Sprintf("[%d,%v,%v]", v.Dimensions, v.Indices, v.Values)
	}
	return fmt.Sprintf("%v", v.Values)
}

// SetVectorValue converts a Go `Vector` into a godror data type.
func SetVectorValue(c *conn, v Vector, data *C.dpiData) error {
	var vectorInfo C.dpiVectorInfo
	var dataPtr unsafe.Pointer
	var format C.uint8_t
	var dimensionSize uint8
	var numDims int

	switch values := v.Values.(type) {
	case []float32:
		numDims = len(values)
		format = C.DPI_VECTOR_FORMAT_FLOAT32
		dimensionSize = 4
		if numDims > 0 {
			dataPtr = unsafe.Pointer(&values[0])
		}
	case []float64:
		numDims = len(values)
		format = C.DPI_VECTOR_FORMAT_FLOAT64
		dimensionSize = 8
		if numDims > 0 {
			dataPtr = unsafe.Pointer(&values[0])
		}
	case []int8:
		numDims = len(values)
		format = C.DPI_VECTOR_FORMAT_INT8
		dimensionSize = 1
		if numDims > 0 {
			ptr := (*C.int8_t)(C.malloc(C.size_t(len(values))))
			cArray := (*[1 << 30]C.int8_t)(unsafe.Pointer(ptr))[:len(values):len(values)]
			for i, v := range values {
				cArray[i] = C.int8_t(v)
			}
			dataPtr = unsafe.Pointer(ptr)
			defer C.free(unsafe.Pointer(ptr))
		}
	case []uint8:
		numDims = len(values)
		format = C.DPI_VECTOR_FORMAT_BINARY
		dimensionSize = 1
		if numDims > 0 {
			ptr := (*C.uint8_t)(C.malloc(C.size_t(len(values))))
			cArray := (*[1 << 30]C.uint8_t)(unsafe.Pointer(ptr))[:len(values):len(values)]
			for i, v := range values {
				cArray[i] = C.uint8_t(v)
			}
			dataPtr = unsafe.Pointer(ptr)
			defer C.free(unsafe.Pointer(ptr))
		}
	default:
		return fmt.Errorf("Unsupported type: %T", v.Values)
	}
	C.setVectorInfoDimensions(&vectorInfo, dataPtr) //update values

	// update sparse indices
	var sparseIndices *C.uint32_t = nil
	var numSparseValues C.uint32_t = 0
	if v.IsSparse {
		if len(v.Indices) > 0 {
			numSparseValues = C.uint32_t(len(v.Indices))
			ptr := (*C.uint32_t)(C.malloc(C.size_t(numSparseValues) * C.size_t(unsafe.Sizeof(C.uint32_t(0)))))
			cArray := (*[1 << 30]C.uint32_t)(unsafe.Pointer(ptr))[:numSparseValues:numSparseValues]
			for i, val := range v.Indices {
				cArray[i] = C.uint32_t(val)
			}
			defer C.free(unsafe.Pointer(ptr))
			sparseIndices = ptr
		}
	} else {
		// update Dimensions for Dense
		var multiplier uint32 = 1
		if format == C.DPI_VECTOR_FORMAT_BINARY {
			// Binary vector is not supported for Sparse
			multiplier = 8 // each byte is 8 dimensions.
		}
		v.Dimensions = multiplier * uint32(numDims)
	}

	// update and set vectorInfo
	vectorInfo.format = format
	vectorInfo.numDimensions = C.uint32_t(v.Dimensions)
	vectorInfo.dimensionSize = C.uint8_t(dimensionSize)
	vectorInfo.numSparseValues = C.uint32_t(numSparseValues)
	vectorInfo.sparseIndices = (*C.uint32_t)(sparseIndices)
	if err := c.checkExec(func() C.int {
		return C.dpiVector_setValue(C.dpiData_getVector(data), &vectorInfo)
	}); err != nil {
		return fmt.Errorf("SetVectorValue %w", err)
	}
	return nil
}

// GetVectorValue converts a C `dpiVectorInfo` struct into a Go `Vector`
func GetVectorValue(vecInfo *C.dpiVectorInfo) (Vector, error) {
	var values interface{}
	var indices []uint32
	var isSparse bool

	var nonZeroVal = vecInfo.numDimensions

	// create Indices
	if vecInfo.numSparseValues > 0 {
		isSparse = true
		nonZeroVal = vecInfo.numSparseValues
		indices = make([]uint32, vecInfo.numSparseValues)
		ptr := (*[1 << 30]C.uint32_t)(unsafe.Pointer(vecInfo.sparseIndices))[:vecInfo.numSparseValues:vecInfo.numSparseValues]
		for i, v := range ptr {
			indices[i] = uint32(v)
		}
	}

	// create Values
	switch vecInfo.format {
	case C.DPI_VECTOR_FORMAT_FLOAT32:
		ptr := (*[1 << 30]float32)(unsafe.Pointer(C.getVectorInfoDimensions(vecInfo)))[:vecInfo.numDimensions:vecInfo.numDimensions]
		values = make([]float32, nonZeroVal)
		copy(values.([]float32), ptr)
	case C.DPI_VECTOR_FORMAT_FLOAT64:
		ptr := (*[1 << 30]float64)(unsafe.Pointer(C.getVectorInfoDimensions(vecInfo)))[:vecInfo.numDimensions:vecInfo.numDimensions]
		values = make([]float64, nonZeroVal)
		copy(values.([]float64), ptr)
	case C.DPI_VECTOR_FORMAT_INT8:
		ptr := (*[1 << 30]int8)(unsafe.Pointer(C.getVectorInfoDimensions(vecInfo)))[:vecInfo.numDimensions:vecInfo.numDimensions]
		values = make([]int8, nonZeroVal)
		copy(values.([]int8), ptr)
	case C.DPI_VECTOR_FORMAT_BINARY:
		size := vecInfo.numDimensions / 8
		ptr := (*[1 << 30]uint8)(unsafe.Pointer(C.getVectorInfoDimensions(vecInfo)))[:size:size]
		values = make([]uint8, size)
		copy(values.([]uint8), ptr)
	default:
		return Vector{}, fmt.Errorf("Unknown VECTOR format: %d", vecInfo.format)
	}

	return Vector{
		Indices:    indices,
		Dimensions: uint32(vecInfo.numDimensions),
		Values:     values,
		IsSparse:   isSparse,
	}, nil
}
