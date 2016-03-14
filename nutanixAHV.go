package nutanixAHV

import (
	"archive/tar"
	//"bytes"
	//"errors"
	"fmt"
	"net/http"
	"io/ioutil"
	"os"
	//"path/filepath"
	
	//"github.com/alexzorin/libvirt-go"
	
	"github.com/Tfindelkind/ntnx-golang-client-sdk"

	"github.com/docker/machine/libmachine/drivers"
	"github.com/docker/machine/libmachine/log"
	"github.com/docker/machine/libmachine/mcnflag"
	"github.com/docker/machine/libmachine/mcnutils"
	"github.com/docker/machine/libmachine/ssh"
	"github.com/docker/machine/libmachine/state"
)

const (
	isoFilename        		= "boot2docker.iso"	
	driverName				= "nutanixAHV"
	defaultUser   			= "admin"
	defaultPass   			= "nutanix/4u"
	defaultMaxCapacityBytes	= "20480"
	defaultMemoryMB	   		= "1024"
	defaultVcpus			= "1" 		
	defaultNetworkName 		= "default"
	defaultContainerName	= "default"
	defaultImageName		= "boot2docker"
	// REMOVE 
	defaultHost			= "192.168.178.41"	
)

const (
	sshUser				= "docker"
	sshPort				= 22
)

type Driver struct {
	*drivers.BaseDriver
	NutanixHost			string
	Username			string
	Password			string
	MemoryMB         	string
	Vcpus            	string
	MaxCapacityBytes 	string
	NetworkName     	string
	ContainerName		string
	ImageName		   	string
	Boot2DockerURL   	string
	vmLoaded			bool
	nc					ntnxAPI.NTNXConnection
	vm					ntnxAPI.VM
	im					ntnxAPI.Image
	vdisk				ntnxAPI.VDisk
	nic					ntnxAPI.Network
}

func (d *Driver) GetCreateFlags() []mcnflag.Flag {
	return []mcnflag.Flag{
		mcnflag.StringFlag{
			Name: 	"nutanixAHV-host",
			Usage: 	"Nutanix Cluster or CVM IP/Name",
			Value: 	defaultHost,
		},		
		mcnflag.StringFlag{
			Name: 	"nutanixAHV-username",
			Usage:  "Nutanix username",
			Value: 	defaultUser,
		},
		mcnflag.StringFlag{
			Name:   "nutanixAHV-password",
			Usage:  "Nutanix password",
			Value: 	defaultPass,
		},
		mcnflag.StringFlag{
			Name:  "nutanixAHV-memory-mb",
			Usage: "Size of memory for host in MB",
			Value: defaultMemoryMB,
		},
		mcnflag.StringFlag{
			Name:  "nutanixAHV-max-capacity-bytes",
			Usage: "Size of disk for host in MB",
			Value: defaultMaxCapacityBytes,
		},
		mcnflag.StringFlag{
			Name:  "nutanixAHV-vcpus",
			Usage: "Number of Vcpus",
			Value: defaultVcpus,
		},
		
		mcnflag.StringFlag{
			Name:  "nutanixAHV-network-name",
			Usage: "Name of network to connect to",
			Value: defaultNetworkName,
		},
		mcnflag.StringFlag{
			Name:  "nutanixAHV-container-name",
			Usage: "Name of container used for vDisk",
			Value: defaultContainerName,
		},
		mcnflag.StringFlag{
			Name:  "nutanixAHV-image-name",
			Usage: "Name of image used for boot",
			Value: defaultImageName,
		},
		mcnflag.StringFlag{
			EnvVar: "NUTANIXAHV_BOOT2DOCKER_URL",
			Name:   "nutanixAHV-boot2docker-url",
			Usage:  "The URL of the boot2docker image. Defaults to the latest available version",
			Value:  "",
		},		
	}
}

func (d *Driver) GetMachineName() string {
	return d.MachineName
}

func (d *Driver) GetSSHHostname() (string, error) {
	return d.GetIP()
}


func (d *Driver) GetSSHPort() (int, error) {
	if d.SSHPort == 0 {
		d.SSHPort = sshPort
	}

	return d.SSHPort, nil
}

func (d *Driver) GetSSHUsername() string {
	if d.SSHUser == "" {
		d.SSHUser = sshUser
	}

	return d.SSHUser
}

func (d *Driver) DriverName() string {
	return driverName
}

func (d *Driver) SetConfigFromFlags(flags drivers.DriverOptions) error {
	log.Debugf("SetConfigFromFlags called")
	
	d.NutanixHost= flags.String("nutanixAHV-host")
	d.Username = flags.String("nutanixAHV-username")
	d.Password = flags.String("nutanixAHV-password")
	d.MemoryMB = flags.String("nutanixAHV-memory-mb")
	d.MaxCapacityBytes = flags.String("nutanixAHV-max-capacity-bytes")
	d.Vcpus = flags.String("nutanixAHV-vcpus")
	d.NetworkName = flags.String("nutanixAHV-network-name")
	d.ContainerName = flags.String("nutanixAHV-container-name")
	d.ImageName = flags.String("nutanixAHV-image-name")
	
	d.Boot2DockerURL = flags.String("nutanixAHV-boot2docker-url")
	
	d.SSHUser = sshUser
	d.SSHPort = sshPort
	return nil
}

func (d *Driver) GetURL() (string, error) {
	log.Debugf("GetURL called")
	ip, err := d.GetIP()
	if err != nil {
		log.Warnf("Failed to get IP: %s", err)
		return "", err
	}
	if ip == "" {
		return "", nil
	}
	return fmt.Sprintf("tcp://%s:2376", ip), nil // TODO - don't hardcode the port!
}

func (d *Driver) PreCreateCheck() error {
	// TODO We could look at d.conn.GetCapabilities()
	// parse the XML, and look for kvm
	log.Debug("About to check libvirt version")

	// TODO might want to check minimum version
	/*_, err := d.conn.GetLibVersion()
	if err != nil {
		log.Warnf("Unable to get libvirt version")
		return err
	}
	err = d.validatePrivateNetwork()
	if err != nil {
		return err
	}
	err = d.validateNetwork(d.Network)
	if err != nil {
		return err
	}
	// Others...? */
	return nil
}

func (d *Driver) Create() error {
	b2dutils := mcnutils.NewB2dUtils(d.StorePath)
	if err := b2dutils.CopyIsoToMachineDir(d.Boot2DockerURL, d.MachineName); err != nil {
		return err
	}

	log.Infof("Creating SSH key..."+d.GetSSHKeyPath())
	if err := ssh.GenerateSSHKey(d.GetSSHKeyPath()); err != nil {
		return err
	}
	
	fmt.Println(d.GetSSHKeyPath()+".pub")
	


	//if err := os.MkdirAll(d.ResolveStorePath("."), 0755); err != nil {
	//	return err
	//}

	log.Infof("Setup Nutanix REST connection...")
	
	
	d.nc = ntnxAPI.NTNXConnection { defaultHost, defaultUser, defaultPass, "",  http.Client{}}
		 	
	ntnxAPI.EncodeCredentials(&d.nc)
	ntnxAPI.CreateHttpClient(&d.nc)
	
	log.Infof("Creating VM...")
	
	d.vm = ntnxAPI.VM { d.MemoryMB , d.MachineName, d.Vcpus, d.NetworkName, ""}
	
	if (ntnxAPI.VMExist(&d.nc,&d.vm)) {
		 fmt.Println("VM already exists")
		} else {
			ntnxAPI.CreateVM(&d.nc,&d.vm)		
	}
	
	fmt.Println("Insert boot2docker into CDROM ...")
	
	d.vm.UUID = ntnxAPI.GetVMIDbyName(&d.nc,d.MachineName)
	
	log.Infof("Insert boot2docker into CDROM ...")
	
	fmt.Println("Insert boot2docker into CDROM ...")
	
	d.im = ntnxAPI.Image { d.ImageName, "", "ISO_IMAGE", "",  ntnxAPI.GetImageIDbyName(&d.nc,d.ImageName)}
	
	ntnxAPI.CloneCDforVM(&d.nc,&d.vm,&d.im)
	
	log.Infof("Creating VM data disk...")
	
	// create vdisk struct with some empty values which will be set later
	d.vdisk = ntnxAPI.VDisk { d.ContainerName, ntnxAPI.GetContainerIDbyName(&d.nc,d.ContainerName), "", d.MaxCapacityBytes,"",false }
	
	ntnxAPI.CreateVDiskforVM(&d.nc,&d.vm,&d.vdisk)
	 
	log.Debugf("Creating VM network...")
	
	// create network struct with some empty values which will be set later
	d.nic = ntnxAPI.Network { d.NetworkName, ntnxAPI.GetNetworkIDbyName(&d.nc, d.NetworkName), 0 }
	
	ntnxAPI.CreateVNicforVM(&d.nc,&d.vm,&d.nic)
	
	log.Infof("Provisioning certs and ssh keys...")
	// Generate a tar keys bundle
	if err := d.generateKeyBundle(); err != nil {
		return err
	}

	//uploading ssh keybundle tar xf /var/lib/boot2docker/userdata.tar -C /home/docker/ > /var/log/userdata.log 2>&1
	
	d.vmLoaded = true
	
	return d.Start()
}

// Make a boot2docker userdata.tar key bundle
func (d *Driver) generateKeyBundle() error {
	log.Debugf("Creating Tar key bundle...")

	magicString := "boot2docker, this is nutanix speaking"

	tf, err := os.Create(d.ResolveStorePath("userdata.tar"))
	if err != nil {
		return err
	}
	defer tf.Close()
	var fileWriter = tf

	tw := tar.NewWriter(fileWriter)
	defer tw.Close()

	// magicString first so we can figure out who originally wrote the tar.
	file := &tar.Header{Name: magicString, Size: int64(len(magicString))}
	if err := tw.WriteHeader(file); err != nil {
		return err
	}
	if _, err := tw.Write([]byte(magicString)); err != nil {
		return err
	}
	// .ssh/key.pub => authorized_keys
	file = &tar.Header{Name: ".ssh", Typeflag: tar.TypeDir, Mode: 0700}
	if err := tw.WriteHeader(file); err != nil {
		return err
	}
	pubKey, err := ioutil.ReadFile(d.publicSSHKeyPath())
	if err != nil {
		return err
	}
	file = &tar.Header{Name: ".ssh/authorized_keys", Size: int64(len(pubKey)), Mode: 0644}
	if err := tw.WriteHeader(file); err != nil {
		return err
	}
	if _, err := tw.Write([]byte(pubKey)); err != nil {
		return err
	}
	file = &tar.Header{Name: ".ssh/authorized_keys2", Size: int64(len(pubKey)), Mode: 0644}
	if err := tw.WriteHeader(file); err != nil {
		return err
	}
	if _, err := tw.Write([]byte(pubKey)); err != nil {
		return err
	}
	if err := tw.Close(); err != nil {
		return err
	}

	return nil

}

func (d *Driver) Start() error {
	log.Debugf("Starting VM %s", d.MachineName)
	d.validateVMRef()
	
	ntnxAPI.StartVM(&d.nc,&d.vm)

	return nil 
}

func (d *Driver) Stop() error {
	
	ntnxAPI.StopVM(&d.nc,&d.vm)
	
	return nil
}

func (d *Driver) Remove() error {

	
	//if (ntnxAPI.VMExist(&d.nc,&d.vm)) {
	//	ntnxAPI.DeleteVM(&d.nc,&d.vm)
	//	}
	return nil
}

func (d *Driver) Restart() error {
	 log.Debugf("Restarting VM %s", d.MachineName)
	if err := d.Stop(); err != nil {
		return err
	}
	return d.Start() 
}

func (d *Driver) Kill() error {
	/*log.Debugf("Killing VM %s", d.MachineName)
	d.validateVMRef()
	return d.VM.Destroy()
	*/
	return nil
}

func (d *Driver) GetState() (state.State, error) {
	log.Debugf("Getting current state...")
	d.validateVMRef()
	states := ntnxAPI.GetVMState(&d.nc,&d.vm)
		
	switch states {
	case "on":
		fmt.Println("on")
		return state.Running, nil
	case "off":
		fmt.Println("off")
		return state.Stopped, nil
	}
	return state.None, nil
}

func (d *Driver) validateVMRef() {
	if !d.vmLoaded {
		log.Debugf("Fetching VM...")
		
		if (!ntnxAPI.VMExist(&d.nc,&d.vm)) {
		 fmt.Println("VM does not exist")
		} 
	}
	 	
}

func (d *Driver) GetIP() (string, error) {
	log.Debugf("GetIP called for %s", d.MachineName)

	ip, err := ntnxAPI.GetVMIP(&d.nc,&d.vm)
	return ip, err
}

func (d *Driver) publicSSHKeyPath() string {
	return d.GetSSHKeyPath() + ".pub"
}


func (d *Driver) GetSSHKeyPath() string {
	return d.ResolveStorePath("id_rsa")
}


func NewDriver(hostName, storePath string) drivers.Driver {

	return &Driver{	}
	
}
