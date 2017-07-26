/*! \file osn.go
    \brief Custom app to handle talking with digital ocean when we need certain things done

*/

package main

import (
	"fmt"
	"flag"
    "os"
    "io/ioutil"
    "encoding/json"
    
    "github.com/NathanRThomas/harbormaster/libraries"
)

const VER		= "0.4"

type config_t struct {
    DO      libraries.DO_config_t  `json:"digital_ocean"`
    CF      libraries.CF_config_t   `json:"cloud_flare"`
}

//-------------------------------------------------------------------------------------------------------------------------//
//----- PRIVATE FUNCTIONS -------------------------------------------------------------------------------------------------//
//-------------------------------------------------------------------------------------------------------------------------//

/*! \brief Reads in our config file
 */
func readConfig (loc string) (config config_t, err error) {
    //Read in the eggs
    configFile, err := os.Open(loc) //try the file
    
	if err == nil {
        defer configFile.Close()
		jsonParser := json.NewDecoder(configFile)
		err = jsonParser.Decode(&config)
        
        if err == nil {
            if len(config.DO.APIKey) < 1 && len(config.CF.APIKey) < 1 {
                err = fmt.Errorf("No valid api keys found")
            } else if len(config.DO.APIKey) > 0 && len(config.DO.APIKey) < 64 {
                err = fmt.Errorf("Digital Ocean api key appears invalid")
            } else if len(config.CF.APIKey) > 0 && len(config.CF.Email) < 1 {
                err = fmt.Errorf("Cloud Flare requires an email associated with the api key")
            } else if len(config.CF.APIKey) > 0 && len(config.CF.Zone) < 1 {
                err = fmt.Errorf("Cloud Flare requires a zone id associated with it")
            }
        }
	} else {
        err = fmt.Errorf("Unable to open '%s' file :: " + err.Error(), loc)
    }
    return
}

/*! \brief Writes the json out for the file
 */
func writeOutput (loc string, fileOutput libraries.FileOutput_t) (error) {
    data, _ := json.Marshal(fileOutput)
    err := ioutil.WriteFile(loc, data, 0644)
    return err
}

//-------------------------------------------------------------------------------------------------------------------------//
//----- MAIN --------------------------------------------------------------------------------------------------------------//
//-------------------------------------------------------------------------------------------------------------------------//

var minversion string	//this gets passed in from the command line build process, and should be the epoch time

func main() {
	
//----- Handle our Flags --------------------------------------------------------------------------------------------------------------//
    
    //Third Parties
    //fTP_DO          := flag.Bool("digitalocean", true, "Use Digital Ocean for this request")  //we do this anyway
    fTP_CloudFlare  := flag.Bool("cloudflare", false, "Use Cloud Flare for this request")
    
    //Actions
    fCreate     := flag.Bool("c", false, "Create a new node")
    fDelete     := flag.Bool("Dn", false, "Delete a node")
    fResize     := flag.Bool("z", false, "Re-size an existing node")
    fDeleteSub  := flag.Bool("Ds", false, "Delete a sub domain")
    fCreateSub  := flag.Bool("cs", false, "Create a sub domain")
    fFloatingIP := flag.Bool("fip", false, "Sets a floating ip to a node")
    
    fTag        := flag.String("tag", "", "Tag to associate with either a node or a balancer")
    fIP         := flag.String("ip", "", "IP address we're targeting")
    fDomainType := flag.String("t", "A", "Type of domain we're targeting. ie 'A' or 'AAAA' etc")
    fSubDomain  := flag.String("sd", "", "Subdomain name we're targeting. ie 'www'")
    fDomain     := flag.String("d", "", "Domain name we're targeting. ie 'google.com'")
	fNodeID     := flag.Int("node", 0, "Node we're targeting")
    fNodeName   := flag.String("n", "", "Name of the target node")
    fRegion     := flag.String("region", "nyc3", "Slug of the region for the node")
    fSize       := flag.Int("size", 0, "Size of the node in gb")
    fCPUSize    := flag.Int("cpu", 0, "Size of node in cpu's, for high cpu droplets")
    fImage      := flag.String("image", "ubuntu-16-04-x64", "OS image to use for the node")
    fSSHKey     := flag.String("sshKey", "", "SSH Key to use when creating a node")
    
    //Other
    fWriteFile  := flag.Bool("o", false, "Writes output to a local json file")
    fVerbose    := flag.Bool("V", false, "Verbose output")
    fSuperV     := flag.Bool("V+", false, "Super verbose output")
    fVersion    := flag.Bool("v", false, "Version")
    fExample    := flag.Bool("example", false, "Examples")
	
	flag.Parse()
	
    if *fVersion {  //we're just looking for the version of the tool
        fmt.Printf("\nHarborMaster: %s.%s\n\n", VER, minversion)
        os.Exit(0)
    }
    if *fExample {  //output some examples
        fmt.Printf("\nExamples\n")
        fmt.Printf("#1\nTo set a specific node to a floating ip address you could do this:\nharbormaster -node=30871086 -ip=\"192.168.1.2\"\n\n")
        fmt.Printf("You can find the ID of a node in the url in the droplets dashboard\n\n")
        os.Exit(0)
    }
    
    if *fSuperV { *fVerbose = true }    //this overrides
    
    
//----- Initialization --------------------------------------------------------------------------------------------------------------//
    cwd, _ := os.Getwd()
    config, err := readConfig(cwd + "/harbormaster.json")
    
    if err != nil { //this is bad
        fmt.Println(err)
        os.Exit(1)
    }
    
    if *fTP_CloudFlare && len(config.CF.APIKey) < 1 {
        fmt.Println("Cannot user ClourFlare without the api_key set in the harbormaster.json config file")
        os.Exit(3)
    }
    
    do := libraries.DO_c {SuperVerbose: *fSuperV, Verbose: *fVerbose, Config: config.DO}   //digital ocean library
    cf := libraries.CF_c {SuperVerbose: *fSuperV, Verbose: *fVerbose, Config: config.CF}   //clourd flare library
    fileOutput := libraries.FileOutput_t{}
    
    //figure out our size, if set
    targetSize := ""
    if *fSize > 0 && *fCPUSize > 0 {
        fmt.Println("Please use either the -size or -cpu flags.\n-size is for a normal droplet based on ram size\n-cpu is for the higher cpu droplets and is based on cpu count")
        os.Exit(4)
    } else if *fSize > 0 {
        targetSize = fmt.Sprintf("%dgb", *fSize)
    } else if *fCPUSize > 0 {
        targetSize = fmt.Sprintf("c-%d", *fCPUSize)
    }
    
//----- Figure out what we're done --------------------------------------------------------------------------------------------------------------//
    if *fCreate {   //we're creating a new node
        if len(*fNodeName) > 0 {
            if len(targetSize) > 0 {
                fmt.Printf("Creating node: %s with the size %s\n", *fNodeName, targetSize)
                err = do.CreateNode(*fNodeName, *fRegion, *fTag, targetSize, *fImage, *fSSHKey, &fileOutput)
            } else {
                err = fmt.Errorf("Size of node not set.  use the -size or -cpu option")
            }
        } else {
            err = fmt.Errorf("Node name not set.  use the -n option")
        }
    
    } else if *fDelete {    //we want to delete a node
        if len(*fNodeName) > 0 {
            err = do.DeleteNode(*fNodeName)
        } else {
            err = fmt.Errorf("Node name not set.  use the -n option")
        }
    
    } else if *fResize {    //we want to resize a node
        if len(*fNodeName) > 0 {
            if len(targetSize) > 0 {
                err = do.ResizeNode(*fNodeName, targetSize)
            } else {
                err = fmt.Errorf("Size to resize to not set.  use the -size or -cpu option")
            }
        } else {
            err = fmt.Errorf("Node name not set.  use the -n option")
        }
    
    } else if *fDeleteSub { //we want to delete a sub domain
        if len(*fSubDomain) > 0 {
            if *fTP_CloudFlare {
                err = cf.DeleteDomainRecord (*fSubDomain)
            } else {
                if len(*fDomain) > 0 {
                    err = do.DeleteDomainRecord(*fDomain, *fSubDomain)
                } else {
                    err = fmt.Errorf("Domain name not set. use the -d option")
                }
            }
        } else {
            err = fmt.Errorf("Subdomain not set.  use the -sd option")
        }
    
    } else if *fFloatingIP {    //we want to set a floating ip to a node
        if len(*fIP) > 0 {
            if *fNodeID > 0 {
                fmt.Println("Setting floating ip to a node")
                
                existing := 0
                existing, err = do.GetFloatingIP(*fIP)
                if err == nil {
                    if existing != *fNodeID {    //they don't match. So let's update them
                        if *fVerbose { fmt.Println("Node not already assigned.  Updating...") }
                        err = do.AssignFloatingIP(*fIP, *fNodeID)
                    } else {
                        if *fVerbose { fmt.Println("Node already assigned.  No work to do") }
                    }
                }
            } else { err = fmt.Errorf("Node id not set.  use the -node option") }
        } else { err = fmt.Errorf("Floating ip address not set.  use the -ip option") }
    
    } else if *fCreateSub { //create a sub domain
        fmt.Println("Setting domain record")
        if len(*fIP) > 0 && len(*fDomainType) > 0 && len(*fSubDomain) > 0 {
            if *fTP_CloudFlare {
                err = cf.AssignDomainRecord (*fDomainType, *fSubDomain, *fIP)
            } else {
                if len(*fDomain) > 0 {
                    err = do.AssignDomainRecord (*fDomain, *fDomainType, *fSubDomain, *fIP)
                } else {
                    err = fmt.Errorf("Missing command line options for creating a sub-domain\n-d")
                }
            }
        } else {
            err = fmt.Errorf("Missing command line options for creating a sub-domain\n-ip, && -sd")
        }
    
    } else {
        fmt.Println("Invalid flags")
        os.Exit(1)
    }

//----- See if we were successful --------------------------------------------------------------------------------------------------------------//
    if err == nil {
        fmt.Println("Success")
        
        if *fWriteFile {    //we want to output the results to a file
            writeOutput(cwd + "/harbormaster_output.json", fileOutput)
        }
    } else {
        fmt.Println(err)
        os.Exit(2)
    }

}
