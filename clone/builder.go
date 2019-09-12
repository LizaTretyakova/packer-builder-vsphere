package clone

import (
	"context"
	packerCommon "github.com/hashicorp/packer/common"
	"github.com/hashicorp/packer/helper/communicator"
	"github.com/hashicorp/packer/helper/multistep"
	"github.com/hashicorp/packer/packer"
	"github.com/jetbrains-infra/packer-builder-vsphere/common"
	"github.com/jetbrains-infra/packer-builder-vsphere/driver"
)

type Builder struct {
	config *Config
	runner multistep.Runner
}

func (b *Builder) Prepare(raws ...interface{}) ([]string, error) {
	c, warnings, errs := NewConfig(raws...)
	if errs != nil {
		return warnings, errs
	}
	b.config = c

	return warnings, nil
}

func (b *Builder) Run(ctx context.Context, ui packer.Ui, hook packer.Hook) (packer.Artifact, error) {
	state := new(multistep.BasicStateBag)
	state.Put("comm", &b.config.Comm)
	state.Put("hook", hook)
	state.Put("ui", ui)

	var steps []multistep.Step

	steps = append(steps,
		&common.StepConnect{
			Config: &b.config.ConnectConfig,
		},
		&StepCloneVM{
			Config:   &b.config.CloneConfig,
			Location: &b.config.LocationConfig,
			Force:    b.config.PackerConfig.PackerForce,
		},
		&common.StepConfigureHardware{
			Config: &b.config.HardwareConfig,
		},
		&common.StepConfigParams{
			Config: &b.config.ConfigParamsConfig,
		},
	)

	if b.config.Comm.Type != "none" {
		steps = append(steps,
			&common.StepSetBootOrder{
				BootOrder: b.config.RunConfig.BootOrder,
				SetOrder: false,
			},
			&common.StepRun{
				Config:   &b.config.RunConfig,
			},
			&common.StepWaitForIp{
				Config: &b.config.WaitIpConfig,
			},
			&communicator.StepConnect{
				Config:    &b.config.Comm,
				Host:      common.CommHost(b.config.Comm.SSHHost),
				SSHConfig: b.config.Comm.SSHConfigFunc(),
			},
			&packerCommon.StepProvision{},
			&common.StepShutdown{
				Config: &b.config.ShutdownConfig,
			},
			&common.StepClearBootOrder{
				BootOrder: b.config.RunConfig.BootOrder,
				SetOrder: false,
			},
		)
	}

	steps = append(steps,
		&common.StepCreateSnapshot{
			CreateSnapshot: b.config.CreateSnapshot,
		},
		&common.StepConvertToTemplate{
			ConvertToTemplate: b.config.ConvertToTemplate,
		},
	)

	b.runner = packerCommon.NewRunner(steps, b.config.PackerConfig, ui)
	b.runner.Run(ctx, state)

	if rawErr, ok := state.GetOk("error"); ok {
		return nil, rawErr.(error)
	}

	if _, ok := state.GetOk("vm"); !ok {
		return nil, nil
	}
	artifact := &common.Artifact{
		Name: b.config.VMName,
		VM:   state.Get("vm").(*driver.VirtualMachine),
	}
	return artifact, nil
}
