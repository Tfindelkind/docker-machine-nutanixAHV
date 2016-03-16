# docker-machine-nutanixAHV
Nutanix AHV (Acropolis Hypervisor) driver for docker-machine

This driver leverages the new [plugin architecture](https://github.com/docker/machine/issues/1626) being
developed for Docker Machine.

# Quick start instructions

* Install [docker-machine](https://github.com/docker/machine/releases)
* Go to the
  [releases]https://github.com/Tfindelkind/docker-machine-nutanixAHV/releases
  page and download the docker-machine-driver-nutanixAHV binary, putting it
  in your PATH.
* You can now create virtual machines using this driver with
  `docker-machine create -d nutanixAHV myengine0`.

# Dependencies

Nutanix NOS 4.5 or higher is required 
Nutanix CE is supported as well
In this actual release you need to manually upload an actual boot2docker iso to the NutanixAHV and name it "boot2docker"


# Capabilities
* **boot2docker.iso** based images
* **Dual Network**
    * **eth1** - A host private network called **docker-machines** is automatically created to ensure we always have connectivity to the VMs.  The `docker-machine ip` command will always return this IP address which is only accessible from your local system.
    
* **Other Tunables**
    * Virtual CPU count via --
    * Disk size via --kvm-disk-size
    * RAM via --kvm-memory

