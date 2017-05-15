package main

import (
	"github.com/mitchellh/multistep"
	"github.com/hashicorp/packer/packer"
	"strconv"
	"github.com/vmware/govmomi/vim25/types"
	"context"
	"github.com/vmware/govmomi/object"
)

type StepConfigureHW struct{
	config *Config
}

type ConfigParametersFlag struct {
	NumCPUsPtr  *int32
	MemoryMBPtr *int64
}

func DefaultConfigParametersFlag() (c *ConfigParametersFlag) {
	c.NumCPUsPtr, c.MemoryMBPtr = nil, nil
	return
}

func (c *ConfigParametersFlag) isEmpty() bool {
	return c.NumCPUsPtr == nil && c.MemoryMBPtr == nil
}

func (s *StepConfigureHW) Run(state multistep.StateBag) multistep.StepAction {
	vm := state.Get("vm").(*object.VirtualMachine)
	ctx := state.Get("ctx").(context.Context)

	var confSpec types.VirtualMachineConfigSpec
	parametersFlag := DefaultConfigParametersFlag()
	// configure HW
	if s.config.Cpus != "" {
		cpus, err := strconv.Atoi(s.config.Cpus)
		if err != nil {
			state.Put("error", err)
			return multistep.ActionHalt
		}

		confSpec.NumCPUs = int32(cpus)
		parametersFlag.NumCPUsPtr = &(confSpec.NumCPUs)
	}
	if s.config.Ram != "" {
		ram, err := strconv.Atoi(s.config.Ram)
		if err != nil {
			state.Put("error", err)
			return multistep.ActionHalt
		}

		confSpec.MemoryMB = int64(ram)
		parametersFlag.MemoryMBPtr = &(confSpec.MemoryMB)
	}

	ui := state.Get("ui").(packer.Ui)
	if !parametersFlag.isEmpty() {
		ui.Say("configuring virtual hardware...")
		task, err := vm.Reconfigure(ctx, confSpec)
		if err != nil {
			state.Put("error", err)
			return multistep.ActionHalt
		}
		_, err = task.WaitForResult(ctx, nil)
		if err != nil {
			state.Put("error", err)
			return multistep.ActionHalt
		}
	} else {
		ui.Say("skipping the virtual hardware configration...")
	}

	return multistep.ActionContinue
}

func (s *StepConfigureHW) Cleanup(multistep.StateBag) {}
