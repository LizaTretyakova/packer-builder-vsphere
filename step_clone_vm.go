package main

import (
	"github.com/vmware/govmomi"
	"context"
	"github.com/mitchellh/multistep"
	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/govmomi/object"
	"github.com/hashicorp/packer/packer"
	"github.com/vmware/govmomi/find"
	"fmt"
	"net/url"
)

type CloneParameters struct {
	client   *govmomi.Client
	folder   *object.Folder
	resourcePool types.ManagedObjectReference
	vmSrc    *object.VirtualMachine
	ctx      context.Context
	vmName   string
}

type StepCloneVM struct{
	config *Config
	success bool
}

func (s *StepCloneVM) Run(state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packer.Ui)
	ui.Say("start cloning...")

	// Prepare entities: client (authentification), finder, folder, virtual machine
	client, ctx, err := createClient(s.config.Url, s.config.Username, s.config.Password)
	if err != nil {
		ui.Say("error creating client")
		state.Put("error", err)
		return multistep.ActionHalt
	}
	finder, ctx, err := createFinder(ctx, client, s.config.DCName)
	if err != nil {
		ui.Say("error creating finder")
		state.Put("error", err)
		return multistep.ActionHalt
	}
	folder, err := finder.FolderOrDefault(ctx, s.config.FolderName)
	if err != nil {
		ui.Say("error creating folder")
		state.Put("error", err)
		return multistep.ActionHalt
	}
	dc, err := finder.DatacenterOrDefault(ctx, s.config.DCName)
	if err != nil {
		ui.Say("error exploring datacenter")
		state.Put("error", err)
		return multistep.ActionHalt
	}
	vmSrc, err := finder.VirtualMachine(ctx, s.config.Template)
	if err != nil {
		ui.Say("error creating vmSrc")
		state.Put("error", err)
		return multistep.ActionHalt
	}
	pool_ref, err := object.NewSearchIndex(client.Client).FindByInventoryPath(ctx, fmt.Sprintf("%v/host/%v/Resources/%v", dc.Name(), s.config.Host, s.config.ResourcePool))
	if err != nil {
		//return fmt.Errorf("Error reading resource pool: %s", err)
		ui.Say(fmt.Sprintf("Error reading resource pool: %s", err))
		state.Put("error", err)
		return multistep.ActionHalt
	}
	if pool_ref == nil {
		//return fmt.Errorf("Cannot find resource pool %s", pool_name)
		ui.Say(fmt.Sprintf("Cannot find resource pool %s", s.config.ResourcePool))
		state.Put("error", err)
		return multistep.ActionHalt

	}

	vm, err := cloneVM(&CloneParameters{
		client:       client,
		folder:       folder,
		//host:         host.Reference(),
		resourcePool: pool_ref.Reference(),
		vmSrc:        vmSrc,
		ctx:          ctx,
		vmName:       s.config.VMName,
	}, ui)
	if err != nil {
		state.Put("error", err)
		return multistep.ActionHalt
	}

	state.Put("vm", vm)
	state.Put("ctx", ctx)
	s.success = true
	return multistep.ActionContinue
}

func (s *StepCloneVM) Cleanup(state multistep.StateBag) {
	if !s.success {
		return
	}

	_, cancelled := state.GetOk(multistep.StateCancelled)
	_, halted := state.GetOk(multistep.StateHalted)

	if cancelled || halted {
		vm := state.Get("vm").(*object.VirtualMachine)
		ctx := state.Get("ctx").(context.Context)
		ui := state.Get("ui").(packer.Ui)

		ui.Say("destroying VM...")

		task, err := vm.Destroy(ctx)
		if err != nil {
			ui.Error(err.Error())
			return
		}
		_, err = task.WaitForResult(ctx, nil)
		if err != nil {
			ui.Error(err.Error())
			return
		}
	}
}

func cloneVM(params *CloneParameters, ui packer.Ui) (vm *object.VirtualMachine, err error) {
	vm = nil
	err = nil

	// Creating specs for cloning
	ui.Say("Creating specs")
	relocateSpec := types.VirtualMachineRelocateSpec{
		Pool: &(params.resourcePool),
	}

	cloneSpec := types.VirtualMachineCloneSpec{
		Location: relocateSpec,
		PowerOn:  false,
	}

	// Cloning itself
	ui.Say("Cloning itself")
	task, err := params.vmSrc.Clone(params.ctx, params.folder, params.vmName, cloneSpec)
	if err != nil {
		return
	}

	ui.Say("Wait for result")
	info, err := task.WaitForResult(params.ctx, nil)
	if err != nil {
		return
	}

	ui.Say("Before creating vm")
	vm = object.NewVirtualMachine(params.client.Client, info.Result.(types.ManagedObjectReference))
	ui.Say("After creating VM")
	return vm, nil
}

func createClient(URL, username, password string) (*govmomi.Client, context.Context, error) {
	// create context
	ctx := context.TODO() // an empty, default context (for those, who is unsure)

	// create a client
	// (connected to the specified URL,
	// logged in with the username-password)
	u, err := url.Parse(URL) // create a URL object from string
	if err != nil {
		return nil, nil, err
	}
	u.User = url.UserPassword(username, password) // set username and password for automatical authentification
	fmt.Println(u.String())
	client, err := govmomi.NewClient(ctx, u,true) // creating a client (logs in with given uname&pswd)
	if err != nil {
		return nil, nil, err
	}

	return client, ctx, nil
}

func createFinder(ctx context.Context, client *govmomi.Client, dcName string) (*find.Finder, context.Context, error) {
	// Create a finder to search for a vm with the specified name
	finder := find.NewFinder(client.Client, false)
	// Need to specify the datacenter
	if dcName == "" {
		dc, err := finder.DefaultDatacenter(ctx)
		if err != nil {
			return nil, nil, err
		}
		finder.SetDatacenter(dc)
	} else {
		dc, err := finder.Datacenter(ctx, dcName)
		if err != nil {
			return nil, nil, err
		}
		finder.SetDatacenter(dc)
	}
	return finder, ctx, nil
}
