// Copyright ©2025 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package transform

import (
	"math"

	"gonum.org/v1/gonum/mat"
	"gonum.org/v1/gonum/stat"
)

// Umeyama finds the similarity transformation between two sets of points
// that minimizes the mean squared error between them.
//
// The transformation relates two sets of n corresponding points {x_i}
// and {y_i} as:
//
//	y_i ≈ c * R * x_i + t,  i=0,...,n-1
//
// where c is the scale factor, R is the rotation matrix and t is
// the translation vector.
//
// The point sets are represented as two n×m matrices X and Y, where
// m is the number of dimensions and x_i and y_i are stored in the i-th
// row of X and Y, respectively. Typically, m is equal to 2 or 3.
//
// The Umeyama type allows inspecting the variance of the input data
// before computing the transformation, providing flexibility in handling
// degenerate cases. It is recommended to always perform a check before
// computation to prevent numerical instability and/or division
// by zero unless there is a specific reason not to do so.
//
// Example usage:
//
//	u := transform.NewUmeyama(x, y)
//	if u.Var() < myThreshold {
//	    // Handle degenerate case
//	}
//	c, r, t, ok := u.Transform()
//	if !ok {
//	    // Handle failure
//	}
//
// For a simple one-call interface with automatic variance checking, use
// the UmeyamaTransform function instead.
//
// Reference:
// "Least-Squares Estimation of Transformation Parameters Between Two Point Patterns"
// by Shinji Umeyama, IEEE Transactions on Pattern Analysis and Machine Intelligence,
// Vol. 13, No. 4, April 1991, [doi:10.1109/34.88573].
// [doi:10.1109/34.88573]: https://doi.org/10.1109/34.88573
type Umeyama struct {
	x, y     *mat.Dense
	n, m     int
	muX, muY *mat.VecDense
	varX     float64
}

// NewUmeyama creates a new Umeyama similarity transformation calculator
// for the given point sets x and y.
//
// The point sets are represented as two n×m matrices X and Y, where
// m is the number of dimensions and each row represents a point.
// If the dimensions of X and Y are not equal, NewUmeyama will panic.
//
// NewUmeyama computes the means and variance of the input points but does
// not perform the full transformation calculation. Use the Var method to
// inspect the variance, and the Transform method to compute the transformation
// parameters.
func NewUmeyama(x, y *mat.Dense) *Umeyama {
	n, m := x.Dims()
	rowsY, colsY := y.Dims()

	// Check dimensions.
	if n != rowsY || m != colsY {
		panic("transform: dimensions of x and y do not match")
	}

	// Calculate means and variance of x.
	muX := mat.NewVecDense(m, nil)
	muY := mat.NewVecDense(m, nil)

	colX := make([]float64, n)
	colY := make([]float64, n)

	var varX float64

	for j := 0; j < m; j++ {
		mat.Col(colX, j, x)
		mat.Col(colY, j, y)

		meanX, varXj := stat.PopMeanVariance(colX, nil)

		muY.SetVec(j, stat.Mean(colY, nil))
		muX.SetVec(j, meanX)

		varX += varXj
	}

	return &Umeyama{
		x:    x,
		y:    y,
		n:    n,
		m:    m,
		muX:  muX,
		muY:  muY,
		varX: varX,
	}
}

// Var returns the variance of point set x.
//
// This can be used to detect degenerate input cases where the variance
// is too low, which may cause numerical instability and/or division by zero
// in the transformation calculation.
func (u *Umeyama) Var() float64 {
	return u.varX
}

// Transform computes and returns the similarity transformation parameters.
//
// Transform returns the scale factor c, the rotation matrix R, the translation
// vector t, and a boolean ok indicating success. The transformation parameters
// best align the point sets according to Umeyama's algorithm.
//
// If the required SVD fails, Transform will return ok as false.
func (u *Umeyama) Transform() (c float64, r *mat.Dense, t *mat.VecDense, ok bool) {
	// Center the matrices.
	xc := mat.NewDense(u.n, u.m, nil)
	yc := mat.NewDense(u.n, u.m, nil)

	for i := 0; i < u.n; i++ {
		for j := 0; j < u.m; j++ {
			xc.Set(i, j, u.x.At(i, j)-u.muX.AtVec(j))
			yc.Set(i, j, u.y.At(i, j)-u.muY.AtVec(j))
		}
	}

	// Calculate covariance matrix.
	covXY := mat.NewDense(u.m, u.m, nil)
	covXY.Mul(yc.T(), xc)
	covXY.Scale(1/float64(u.n), covXY)

	// Singular Value Decomposition
	var svd mat.SVD
	if !svd.Factorize(covXY, mat.SVDFull) {
		return 0, nil, nil, false
	}

	// Get U and V.
	var uu, v mat.Dense
	svd.UTo(&uu)
	svd.VTo(&v)

	// Create identity matrix.
	s := mat.NewDiagDense(u.m, nil)
	for i := 0; i < u.m; i++ {
		s.SetDiag(i, 1)
	}

	// Check determinants to ensure proper rotation matrix (not reflection).
	if mat.Det(&uu)*mat.Det(&v) < 0 {
		s.SetDiag(u.m-1, -1)
	}

	// Calculate scale factor c.
	singularValues := svd.Values(nil)
	for i := 0; i < u.m; i++ {
		c += singularValues[i] * s.At(i, i)
	}
	c /= u.varX

	// Calculate rotation matrix R.
	r = mat.NewDense(u.m, u.m, nil)
	r.Product(&uu, s, v.T())

	// Calculate translation vector t.
	t = mat.NewVecDense(u.m, nil)
	rMuX := mat.NewVecDense(u.m, nil)
	rMuX.MulVec(r, u.muX)

	t.CopyVec(u.muY)
	t.AddScaledVec(t, -c, rMuX)

	return c, r, t, true
}

// UmeyamaTransform finds the similarity transformation between two sets of points
// that minimizes the mean squared error between them.
//
// The transformation relates two sets of n corresponding points {x_i}
// and {y_i} as:
//
//	y_i ≈ c * R * x_i + t,  i=0,...,n-1
//
// where c is the scale factor, R is the rotation matrix and t is
// the translation vector.
//
// The point sets are represented as two n×m matrices X and Y, where
// m is the number of dimensions and x_i and y_i are stored in the i-th
// row of X and Y, respectively. Typically, m is equal to 2 or 3.
// If the dimensions of X and Y are not equal, UmeyamaTransform will panic.
//
// UmeyamaTransform returns the scale factor c, the rotation matrix R and the translation
// vector t.
//
// If the required SVD fails, UmeyamaTransform will return a mat.ErrFailedSVD.
//
// UmeyamaTransform automatically checks for degenerate input by comparing the variance
// of x with machine epsilon. This is necessary because a variance equal or close
// to zero may cause numerical instability and/or division by zero.
// In case of variance ≤ machine epsilon, UmeyamaTransform will return a DegenerateInputError.
//
// For more control over variance checking, use NewUmeyama to create an Umeyama
// instance, inspect its variance with the Var method, and call Transform if appropriate.
//
// Reference:
// "Least-Squares Estimation of Transformation Parameters Between Two Point Patterns"
// by Shinji Umeyama, IEEE Transactions on Pattern Analysis and Machine Intelligence,
// Vol. 13, No. 4, April 1991, [doi:10.1109/34.88573].
// [doi:10.1109/34.88573]: https://doi.org/10.1109/34.88573
func UmeyamaTransform(x, y *mat.Dense) (c float64, r *mat.Dense, t *mat.VecDense, err error) {
	u := NewUmeyama(x, y)

	if u.varX <= math.Nextafter(1.0, 2.0)-1.0 {
		return 0, nil, nil, DegenerateInputError(u.varX)
	}

	var ok bool
	c, r, t, ok = u.Transform()
	if !ok {
		return 0, nil, nil, ErrFactorizationFailed
	}
	return c, r, t, nil
}
