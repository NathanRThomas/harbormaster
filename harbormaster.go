/*! \file osn.go
    \brief Custom app to handle talking with digital ocean when we need certain things done

*/

package main

import (
	"fmt"
	"flag"
    "os"
    "encoding/json"
    
    "github.com/NathanRThomas/harbormaster/libraries"
)

const VER		= "0.1"

type config_t struct {
    DO      libraries.DO_config_t  `json:"digital_ocean"`
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
            if len(config.DO.APIKey) < 64 {
                err = fmt.Errorf("Digital Ocean api key appears invalid")
            }
        }
	} else {
        err = fmt.Errorf("Unable to open '%s' file :: " + err.Error(), loc)
    }
    return
}

//-------------------------------------------------------------------------------------------------------------------------//
//----- MAIN --------------------------------------------------------------------------------------------------------------//
//-------------------------------------------------------------------------------------------------------------------------//

var minversion string	//this gets passed in from the command line build process, and should be the epoch time

func main() {
	
//----- Handle our Flags --------------------------------------------------------------------------------------------------------------//
    fIP         := flag.String("ip", "", "IP address we're targeting")
    fDomainType := flag.String("t", "A", "Type of domain we're targeting. ie 'A' or 'AAAA' etc")
    fSubDomain  := flag.String("sd", "", "Subdomain name we're targeting. ie 'www'")
    fDomain     := flag.String("d", "", "Domain name we're targeting. ie 'google.com'")
	fNodeID     := flag.Int("node", 0, "Node we're targeting")
    fVerbose    := flag.Bool("V", false, "Verbose output")
    fSuperV     := flag.Bool("V+", false, "Super verbose output")
    fVersion    := flag.Bool("v", false, "Version")
    fExample    := flag.Bool("example", false, "Examples")
	
	flag.Parse()
	
    fmt.Println("DOing it in GO")
    
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
    config, err := readConfig("/var/www/work/bin/harbormaster.json")
    
    if err != nil { //this is bad
        fmt.Println(err)
        os.Exit(1)
    }
    
    do := libraries.DO_c {SuperVerbose: *fSuperV, Verbose: *fVerbose, Config: config.DO}   //digital ocean library

//----- Figure out what we're done --------------------------------------------------------------------------------------------------------------//
    if len(*fIP) > 0 && *fNodeID > 0 {
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
    } else if len(*fIP) > 0 && len(*fDomainType) > 0 && len(*fSubDomain) > 0 && len(*fDomain) > 0 {
        fmt.Println("Setting domain record")
        err = do.AssignDomainRecord (*fDomain, *fDomainType, *fSubDomain, *fIP)
    } else {
        fmt.Println("Invalid flags")
        os.Exit(1)
    }

//----- See if we were successful --------------------------------------------------------------------------------------------------------------//
    if err == nil {
        fmt.Println("Success")
    } else {
        fmt.Println(err)
        os.Exit(2)
    }

}
