package main

import (
	"github.com/tfindelkind/docker-machine-nutanixAHV"
	"github.com/docker/machine/libmachine/drivers/plugin"
)

func main() {
	plugin.RegisterDriver(nutanix.NewDriver("default", "path"))
}
