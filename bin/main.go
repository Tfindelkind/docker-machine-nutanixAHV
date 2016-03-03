package main

import (
	"github.com/Tfindelkind/docker-machine-nutanixAHV"
	"github.com/docker/machine/libmachine/drivers/plugin"
)

func main() {
	plugin.RegisterDriver(nutanixAHV.NewDriver("default", "path"))
}
