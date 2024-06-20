package hclparse

import "github.com/hashicorp/hcl/v2"

type ModuleOutputs struct {
	Name 	string 		`hcl:"name,label"`
	Description *string `hcl:"description"`
	Sensitive *bool 	`hcl:"sensitive"`
	Remain  hcl.Body	`hcl:",remain"`
}

type ModuleInputs struct {
	Name        string  `hcl:"name,label"`
	Description *string `hcl:"description"`
	Default     *string `hcl:"default"`
	Remain  hcl.Body    `hcl:",remain"`
}

type ModuleConfig struct {
	Inputs  []ModuleInputs  `hcl:"variable,block"`
	Outputs []ModuleOutputs `hcl:"output,block"`
	Remain  hcl.Body        `hcl:",remain"`
}