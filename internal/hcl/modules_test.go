package hcl

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParsing(t *testing.T) {
	// TestVariableParsing tests the parsing of variables from a module
	// config file
	const TYPE_DEFAULT_STRING = `
		variable "sample_name" {
			type = string
			default = "sample"
		}
	`

	const TYPE_DEFAULT_INT = `
		variable "sample_int" {
			type = int
			default = 1
	}
	`
	const TYPE_DEFAULT_BOOL = `
		variable "sample_bool" {
			type = bool
			default = true
		}
	`

	const TYPE_VAR_LIST = `
		variable "sample_list" {
			type = list(string)
			default = ["sample1", "sample2"]
		}
	`
	const TYPE_VAR_MAP = `
		variable "sample_map" {
			type = map(string)
			default = {
				key1 = "value1"
				key2 = "value2"
			}
		}
	`

	const TYPE_VAR_MAP_EMPTY = `
		variable "sample_map" {
			type = map(string)
			default = {}
		}
	`

	t.Run("Test input of string type", func(t *testing.T) {
		// TestVariableParsing tests the parsing of variables from a module
		// config file
		var moduleConfig ModuleConfig
		err := ParseHCLBytes([]byte(TYPE_DEFAULT_STRING), &moduleConfig)
		if err != nil {
			t.Error(err)
		}
		fmt.Println(moduleConfig.Inputs[0].Default)
		assert.Equal(t, 1, len(moduleConfig.Inputs))
		assert.Nil(t, err)
	})

	t.Run("Test input of list type", func(t *testing.T) {
		var moduleConfig ModuleConfig
		err := ParseHCLBytes([]byte(TYPE_VAR_LIST), &moduleConfig)
		if err != nil {
			t.Error(err)
		}
		assert.Equal(t, 1, len(moduleConfig.Inputs))
		assert.Nil(t, err)
	})

	t.Run("Test input of map type", func(t *testing.T) {
		var moduleConfig ModuleConfig
		err := ParseHCLBytes([]byte(TYPE_VAR_MAP), &moduleConfig)
		if err != nil {
			t.Error(err)
		}
		assert.Equal(t, 1, len(moduleConfig.Inputs))
		assert.Nil(t, err)
	})

	t.Run("Test input of int type", func(t *testing.T) {
		var moduleConfig ModuleConfig
		err := ParseHCLBytes([]byte(TYPE_DEFAULT_INT), &moduleConfig)
		if err != nil {
			t.Error(err)
		}
		assert.Equal(t, 1, len(moduleConfig.Inputs))
		assert.Nil(t, err)
	})

	t.Run("Test input of bool type", func(t *testing.T) {
		var moduleConfig ModuleConfig
		err := ParseHCLBytes([]byte(TYPE_DEFAULT_BOOL), &moduleConfig)
		if err != nil {
			t.Error(err)
		}
		assert.Equal(t, 1, len(moduleConfig.Inputs))
		assert.Nil(t, err)
	})

	t.Run("Test input of empty map type", func(t *testing.T) {
		var moduleConfig ModuleConfig
		err := ParseHCLBytes([]byte(TYPE_VAR_MAP_EMPTY), &moduleConfig)
		if err != nil {
			t.Error(err)
		}
		assert.Equal(t, 1, len(moduleConfig.Inputs))
		assert.Nil(t, err)
	})

}
