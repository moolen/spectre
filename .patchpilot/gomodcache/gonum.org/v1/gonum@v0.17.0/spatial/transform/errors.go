// Copyright ©2025 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package transform

import (
	"errors"
	"fmt"
)

// DegenerateInputError represents an error due to input data with variance
// below a threshold, which would cause numerical instability.
type DegenerateInputError float64

func (e DegenerateInputError) Error() string {
	return fmt.Sprintf("variance too low: %v", float64(e))
}

var ErrFactorizationFailed = errors.New("transform: factorization failed")
