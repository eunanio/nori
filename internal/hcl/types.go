package hcl

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
)

type ModuleOutputs struct {
	Name        string   `hcl:"name,label"`
	Description *string  `hcl:"description"`
	Sensitive   *bool    `hcl:"sensitive"`
	Remain      hcl.Body `hcl:",remain"`
}

type ModuleInputs struct {
	Name         string     `hcl:"name,label" cty:"name"`
	Description  *string    `hcl:"description" cty:"description"`
	Default      *cty.Value `hcl:"default"`
	DefaultValue interface{}
	Remain       hcl.Body `hcl:",remain"`
}

type ModuleResources struct {
	Type   string   `hcl:"type,label"`
	Name   string   `hcl:"name,label"`
	Remain hcl.Body `hcl:",remain"`
}

type ModuleConfig struct {
	Inputs    []ModuleInputs    `hcl:"variable,block"`
	Outputs   []ModuleOutputs   `hcl:"output,block"`
	Resources []ModuleResources `hcl:"resource,block"`
	Remain    hcl.Body          `hcl:",remain"`
}
