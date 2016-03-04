package nutanixAHV

import (
	//"archive/tar"
	//"bytes"
	//"errors"
	"fmt"
	"net/http"
	"io"
	//"io/ioutil"
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
	ISO              	string
	Boot2DockerURL   	string
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

func (d *Driver) GetSSHKeyPath() string {
	return d.ResolveStorePath("id_rsa")
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
	
	d.NutanixHost= flags.String("nutanixAHV-nutanix-host")
	d.Username = flags.String("nutanixAHV-username")
	d.Password = flags.String("nutanixAHV-password")
	d.MemoryMB = flags.String("nutanixAHV-memory-mb")
	d.MaxCapacityBytes = flags.String("nutanixAHV-max-capacity-bytes")
	d.Vcpus = flags.String("nutanixAHV-vcpus")
	d.NetworkName = flags.String("nutanixAHV-network-name")
	d.ContainerName = flags.String("nutanixAHV-container-name")
	
	d.Boot2DockerURL = flags.String("nutanixAHV-boot2docker-url")

	d.SwarmMaster = flags.Bool("swarm-master")
	d.SwarmHost = flags.String("swarm-host")
	d.SwarmDiscovery = flags.String("swarm-discovery")
	d.ISO = d.ResolveStorePath(isoFilename)
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

// Create, or verify the private network is properly configured
func (d *Driver) validatePrivateNetwork() error {
	log.Debug("Validating private network")
	/*network, err := d.conn.LookupNetworkByName(d.PrivateNetwork)
	if 1 == 2 {
		xmldoc, err := network.GetXMLDesc(0)
		if err != nil {
			return err
		}
		/* XML structure:
		<network>
		    ...
		    <ip address='a.b.c.d' netmask='255.255.255.0'>
		        <dhcp>
		            <range start='a.b.c.d' end='w.x.y.z'/>
		        </dhcp>

		type Ip struct {
			Address string `xml:"address,attr"`
			Netmask string `xml:"netmask,attr"`
		}
		type Network struct {
			Ip Ip `xml:"ip"`
		}

		var nw Network
		err = xml.Unmarshal([]byte(xmldoc), &nw)
		if err != nil {
			return err
		}

		if nw.Ip.Address == "" {
			return fmt.Errorf("%s network doesn't have DHCP configured properly", d.PrivateNetwork)
		}
		// Corner case, but might happen...
		if active, err := network.IsActive(); !active {
			log.Debugf("Reactivating private network: %s", err)
			err = network.Create()
			if err != nil {
				log.Warnf("Failed to Start network: %s", err)
				return err
			}
		}
		return nil
	}
	// TODO - try a couple pre-defined networks and look for conflicts before
	//        settling on one
	xml := fmt.Sprintf(networkXML, d.PrivateNetwork,
		"192.168.42.1",
		"255.255.255.0",
		"192.168.42.2",
		"192.168.42.254")
	//network, err = d.conn.NetworkDefineXML(xml)
	if err != nil {
		log.Errorf("Failed to create private network: %s", err)
		return nil
	}
	err = network.SetAutostart(true)
	if err != nil {
		log.Warnf("Failed to set private network to autostart: %s", err)
	}
	err = network.Create()
	if err != nil {
		log.Warnf("Failed to Start network: %s", err)
		return err
	} */
	return nil
}

func (d *Driver) validateNetwork(name string) error {
	log.Debugf("Validating network %s", name)
	/*_, err := d.conn.LookupNetworkByName(name)
	if err != nil {
		log.Errorf("Unable to locate network %s", name)
		return err
	}*/
	return nil
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

	log.Infof("Creating SSH key...")
	if err := ssh.GenerateSSHKey(d.GetSSHKeyPath()); err != nil {
		return err
	}

	if err := os.MkdirAll(d.ResolveStorePath("."), 0755); err != nil {
		return err
	}

	/* Libvirt typically runs as a deprivileged service account and
	// needs the execute bit set for directories that contain disks
	for dir := d.ResolveStorePath("."); dir != "/"; dir = filepath.Dir(dir) {
		log.Debugf("Verifying executable bit set on %s", dir)
		info, err := os.Stat(dir)
		if err != nil {
			return err
		}
		mode := info.Mode()
		if mode&0001 != 1 {
			log.Debugf("Setting executable bit set on %s", dir)
			mode |= 0001
			os.Chmod(dir, mode)
		}
	}*/

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
	
	ntnxAPI.GetVMIDbyName(&d.nc,d.MachineName)
	
	log.Debugf("Creating VM data disk...")
	
	d.vdisk = ntnxAPI.VDisk { d.ContainerName, ntnxAPI.GetContainerIDbyName(&d.nc,d.ContainerName), "", d.MaxCapacityBytes,"",false }
	
	ntnxAPI.CreateVDiskforVM(&d.nc,&d.vm,&d.vdisk)
	 
	//ntnxAPI.GetNetworkID(&n,&nic1)
	//ntnxAPI.CreateVNicforVM(&n, &v,&nic1)
	/*if err := d.generateDiskImage(d.DiskSize); err != nil {
		return err
	}*/

	log.Debugf("Defining VM...")
	/*tmpl, err := template.New("domain").Parse(domainXMLTemplate)
	if err != nil {
		return err
	}
	var xml bytes.Buffer
	err = tmpl.Execute(&xml, d)
	if err != nil {
		return err
	}*/

	/*vm, err := d.conn.DomainDefineXML(xml.String())
	if err != nil {
		log.Warnf("Failed to create the VM: %s", err)
		return err
	}
	 
	d.VM = &vm
	d.vmLoaded = true
	*/
	return d.Start()
}

func (d *Driver) Start() error {
	log.Debugf("Starting VM %s", d.MachineName)
	d.validateVMRef()
	/*err := d.VM.Create()
	if err != nil {
		log.Warnf("Failed to start: %s", err)
		return err
	}

	// They wont start immediately
	time.Sleep(5 * time.Second)

	for i := 0; i < 90; i++ {
		time.Sleep(time.Second)
		ip, _ := d.GetIP()
		if ip != "" {
			// Add a second to let things settle
			time.Sleep(time.Second)
			return nil
		}
		log.Debugf("Waiting for the VM to come up... %d", i)
	}
	log.Warnf("Unable to determine VM's IP address, did it fail to boot?") */
	return nil 
}

func (d *Driver) Stop() error {
	/*log.Debugf("Stopping VM %s", d.MachineName)
	d.validateVMRef()
	s, err := d.GetState()
	if err != nil {
		return err
	}

	if s != state.Stopped {
		err := d.VM.DestroyFlags(libvirt.VIR_DOMAIN_DESTROY_GRACEFUL)
		if err != nil {
			log.Warnf("Failed to gracefully shutdown VM")
			return err
		}
		for i := 0; i < 90; i++ {
			time.Sleep(time.Second)
			s, _ := d.GetState()
			log.Debugf("VM state: %s", s)
			if s == state.Stopped {
				return nil
			}
		}
		return errors.New("VM Failed to gracefully shutdown, try the kill command")
	}*/
	return nil
}

func (d *Driver) Remove() error {
	/*log.Debugf("Removing VM %s", d.MachineName)
	d.validateVMRef()
	// Note: If we switch to qcow disks instead of raw the user
	//       could take a snapshot.  If you do, then Undefine
	//       will fail unless we nuke the snapshots first
	// d.VM.Destroy() // Ignore errors
	return d.VM.Undefine() */
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
	/*log.Debugf("Getting current state...")
	d.validateVMRef()
	states, err := d.VM.GetState()
	if err != nil {
		return state.None, err
	}
	switch states[0] {
	case libvirt.VIR_DOMAIN_NOSTATE:
		return state.None, nil
	case libvirt.VIR_DOMAIN_RUNNING:
		return state.Running, nil
	case libvirt.VIR_DOMAIN_BLOCKED:
		// TODO - Not really correct, but does it matter?
		return state.Error, nil
	case libvirt.VIR_DOMAIN_PAUSED:
		return state.Paused, nil
	case libvirt.VIR_DOMAIN_SHUTDOWN:
		return state.Stopped, nil
	case libvirt.VIR_DOMAIN_CRASHED:
		return state.Error, nil
	case libvirt.VIR_DOMAIN_PMSUSPENDED:
		return state.Saved, nil
	case libvirt.VIR_DOMAIN_SHUTOFF:
		return state.Stopped, nil
	}*/
	return state.None, nil
}

func (d *Driver) validateVMRef() {
	/*if !d.vmLoaded {
		log.Debugf("Fetching VM...")
		vm, err := d.conn.LookupDomainByName(d.MachineName)
		if err != nil {
			log.Warnf("Failed to fetch machine")
		} else {
			d.VM = &vm
			d.vmLoaded = true
		}
	}*/
}

// This implementation is specific to default networking in libvirt
// with dnsmasq
func (d *Driver) getMAC() (string, error) {
	/*d.validateVMRef()
	xmldoc, err := d.VM.GetXMLDesc(0)
	if err != nil {
		return "", err
	}
	/* XML structure:
	<domain>
	    ...
	    <devices>
	        ...
	        <interface type='network'>
	            ...
	            <mac address='52:54:00:d2:3f:ba'/>
	            ...
	        </interface>
	        ...
	
	type Mac struct {
		Address string `xml:"address,attr"`
	}
	type Source struct {
		Network string `xml:"network,attr"`
	}
	type Interface struct {
		Type   string `xml:"type,attr"`
		Mac    Mac    `xml:"mac"`
		Source Source `xml:"source"`
	}
	type Devices struct {
		Interfaces []Interface `xml:"interface"`
	}
	type Domain struct {
		Devices Devices `xml:"devices"`
	}

	var dom Domain
	err = xml.Unmarshal([]byte(xmldoc), &dom)
	if err != nil {
		return "", err
	}
	// Always assume the second interface is the one we want
	if len(dom.Devices.Interfaces) < 2 {
		return "", fmt.Errorf("VM doesn't have enough network interfaces.  Expected at least 2, found %d",
			len(dom.Devices.Interfaces))
	}
	return dom.Devices.Interfaces[1].Mac.Address, nil  */
	return "", nil
}

func (d *Driver) getIPByMACFromLeaseFile(mac string) (string, error) {
	/*leaseFile := fmt.Sprintf(dnsmasqLeases, d.PrivateNetwork)
	data, err := ioutil.ReadFile(leaseFile)
	if err != nil {
		log.Debugf("Failed to retrieve dnsmasq leases from %s", leaseFile)
		return "", err
	}
	for lineNum, line := range strings.Split(string(data), "\n") {
		if len(line) == 0 {
			continue
		}
		entries := strings.Split(line, " ")
		if len(entries) < 3 {
			log.Warnf("Malformed dnsmasq line %d", lineNum+1)
			return "", errors.New("Malformed dnsmasq file")
		}
		if strings.ToLower(entries[1]) == strings.ToLower(mac) {
			log.Debugf("IP address: %s", entries[2])
			return entries[2], nil
		}
	} */
	return "", nil
}

func (d *Driver) getIPByMacFromSettings(mac string) (string, error) {
	/*network, err := d.conn.LookupNetworkByName(d.PrivateNetwork)
	if err != nil {
		log.Warnf("Failed to find network: %s", err)
		return "", err
	}
	bridge_name, err := network.GetBridgeName()
	if err != nil {
		log.Warnf("Failed to get network bridge: %s", err)
		return "", err
	}
	statusFile := fmt.Sprintf(dnsmasqStatus, bridge_name)
	data, err := ioutil.ReadFile(statusFile)
	type Lease struct {
		Ip_address  string `json:"ip-address"`
		Mac_address string `json:"mac-address"`
		// Other unused fields omitted
	}
	var s []Lease

	err = json.Unmarshal(data, &s)
	if err != nil {
		log.Warnf("Failed to decode dnsmasq lease status: %s", err)
		return "", err
	}
	for _, value := range s {
		if strings.ToLower(value.Mac_address) == strings.ToLower(mac) {
			log.Debugf("IP address: %s", value.Ip_address)
			return value.Ip_address, nil
		}
	} */
	return "", nil
}

func (d *Driver) GetIP() (string, error) {
	log.Debugf("GetIP called for %s", d.MachineName)
	mac, err := d.getMAC()
	if err != nil {
		return "", err
	}
	/*
	 * TODO - Figure out what version of libvirt changed behavior and
	 *        be smarter about selecting which algorithm to use
	 */
	ip, err := d.getIPByMACFromLeaseFile(mac)
	if ip == "" {
		ip, err = d.getIPByMacFromSettings(mac)
	}
	log.Debugf("Unable to locate IP address for MAC %s", mac)
	return ip, err
}

func (d *Driver) publicSSHKeyPath() string {
	return d.GetSSHKeyPath() + ".pub"
}



// createDiskImage makes a disk image at dest with the given size in MB. If r is
// not nil, it will be read as a raw disk image to convert from.
func createDiskImage(dest string, size int, r io.Reader) error {
	// Convert a raw image from stdin to the dest VMDK image.
	sizeBytes := int64(size) << 20 // usually won't fit in 32-bit int (max 2GB)
	f, err := os.Create(dest)
	if err != nil {
		return err
	}

	_, err = io.Copy(f, r)
	if err != nil {
		return err
	}
	// Rely on seeking to create a sparse raw file for qemu
	f.Seek(sizeBytes-1, 0)
	f.Write([]byte{0})
	return f.Close()
}

func NewDriver(hostName, storePath string) drivers.Driver {
	/*conn, err := libvirt.NewVirConnection(connectionString)
	if err != nil {
		log.Errorf("Failed to connect to libvirt: %s", err)
		os.Exit(1)
	}*/

	return &Driver{	}
	
}
