/*
Copyright 2022 The Kubeforce Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package rand

import (
	"math/rand"
	"time"
)

// This is a set of characters that can be used in the string generator.
const (
	LowerCaseLetters = "abcdefghijklmnopqrstuvwxyz"
	UpperCaseLetters = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	Numbers          = "0123456789"
	Charset          = LowerCaseLetters + UpperCaseLetters + Numbers
)

// StringWithCharset returns the generated string.
func StringWithCharset(length int, charset string) string {
	//nolint:gosec
	seededRand := rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}

// String returns a generated string of a certain length, using the entire character set.
func String(length int) string {
	return StringWithCharset(length, Charset)
}
