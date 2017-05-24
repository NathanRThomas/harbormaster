/*! \file do.go
    \brief Our generic class for a wrapper around the digital ocean stuff
*/

package libraries

import (
    "fmt"
    "net/http"
    "io/ioutil"
    "bytes"
    "encoding/json"
    "strings"
    "time"
    )

//-------------------------------------------------------------------------------------------------------------------------//
//----- CONSTS ------------------------------------------------------------------------------------------------------------//
//-------------------------------------------------------------------------------------------------------------------------//

const do_base_url          = "https://api.digitalocean.com/v2/"

//-------------------------------------------------------------------------------------------------------------------------//
//----- STRUCTS -----------------------------------------------------------------------------------------------------------//
//-------------------------------------------------------------------------------------------------------------------------//

type DO_config_t struct {
    APIKey  string  `json:"api_key"`
}

type do_t struct {
    Type    string `json:"type,omitempty"`
    ID      int    `json:"droplet_id,omitempty"`
    Size    string  `json:"size,omitempty"`
}

type do_floating_t struct {
    FloatingIP  struct {
        Droplet struct {
            ID  int     `json:"id"`
        } `json:"droplet"`
    } `json:"floating_ip"`
}

type do_domain_record_t struct {
    ID      int     `json:"id,omitempty"`
    Type    string  `json:"type"`
    Name    string  `json:"name"`
    Data    string  `json:"data,omitempty"`
}

type do_network_t struct {
    IP      string  `json:"ip_address"`
    Netmask string  `json:"netmask"`
    Gateway string  `json:"gateway"`
    Type    string  `json:"public"`
}

type do_droplet_t struct {
    ID      int     `json:"id"`
    Name    string  `json:"name"`
    Memory  int     `json:"memory"`
    Status  string  `json:"status"`
    Locked  bool    `json:"locked"`
    
    Networks struct {
        V4 []do_network_t   `json:"v4"`
    }   `json:"networks"`
}

type FileOutput_t struct {
    Droplet     do_droplet_t    `json:"droplet"`
}

type DO_c struct {
    Verbose, SuperVerbose     bool
    Config      DO_config_t
}

//-------------------------------------------------------------------------------------------------------------------------//
//----- PRIVATE FUNCTIONS -------------------------------------------------------------------------------------------------//
//-------------------------------------------------------------------------------------------------------------------------//

func (do DO_c) request (url string, jStr []byte) (body []byte, err error) {
    var req *http.Request
    
    if len(jStr) > 0 {    //we're posting data
        req, err = http.NewRequest("POST", do_base_url + url, bytes.NewBuffer(jStr))
    } else {    //we're doing a get
        req, err = http.NewRequest("GET", do_base_url + url, nil)
    }
    if err == nil {
        req.Header.Set("Content-Type", "application/json")
        req.Header.Set("Authorization", "Bearer " + do.Config.APIKey)
        
        client := &http.Client{}
        resp, err := client.Do(req)
        if err == nil {
            defer resp.Body.Close()
            
            body, _ = ioutil.ReadAll(resp.Body)
            
            if do.SuperVerbose {
                fmt.Println("response Status:", resp.Status)
                fmt.Println("response Headers:", resp.Header)
                fmt.Println("response Body:", string(body[:]))
            }
        } else {
            return nil, err
        }
    }
    
    return
}

/*! \brief For when we do a delete request where we aren't expecting a body, only a return code
 */
func (do DO_c) deleteRequest (url string) (err error) {
    req, err := http.NewRequest("DELETE", do_base_url + url, nil)
    
    if err == nil {
        req.Header.Set("Content-Type", "application/json")
        req.Header.Set("Authorization", "Bearer " + do.Config.APIKey)
        
        client := &http.Client{}
        resp, err := client.Do(req)
        if err == nil {
            defer resp.Body.Close()
            if do.SuperVerbose {
                fmt.Println("response Status:", resp.Status)
                fmt.Println("response Headers:", resp.Header)
            }
            
            if resp.StatusCode != 204 {
                return fmt.Errorf("Delete request failed: status code: %d - url: %s", resp.StatusCode, url)
            }
        } else {
            return err
        }
    }
    
    return
}

/*! \brief Creates a domain record when one doesn't exist yet
 */
func (do DO_c) createDomainRecord (domain, domainType, subDomain, ip string) (err error) {
    record := do_domain_record_t{Type: domainType, Name: subDomain, Data: ip}
    jStr, _ := json.Marshal(record)
    _, err = do.request(fmt.Sprintf("domains/%s/records", domain), jStr)
    return
}


/*! \brief Handles shutting down and powering off a node
 */
func (do DO_c) shutdownNode (droplet *do_droplet_t) (err error) {
    simple := do_t{Type: "shutdown"}
    jStr, _ := json.Marshal(simple)
    if do.Verbose { fmt.Println("Shutting down node") }
    _, err = do.request(fmt.Sprintf("droplets/%d/actions", droplet.ID), jStr)   //issue the shutdown command
    
    if err == nil {
        off := do.waitForNodeStatus(droplet.ID, "off", 10) //wait for this to be off, or we can bail
        if !off {   //this didn't work, hit it with the hammah
            simple.Type = "power_off"
            jStr, _ = json.Marshal(simple)
            if do.Verbose { fmt.Println("Powering OFF node") }
            _, err = do.request(fmt.Sprintf("droplets/%d/actions", droplet.ID), jStr)   //issue the poweroff command
            time.Sleep(time.Second * 5)
        }
    }
    return
}

/*! \brief Start up the node
 */
func (do DO_c) startNode (droplet *do_droplet_t) (err error) {
    simple := do_t{Type: "power_on"}
    jStr, _ := json.Marshal(simple)
    if do.Verbose { fmt.Println("Powering ON node") }
    _, err = do.request(fmt.Sprintf("droplets/%d/actions", droplet.ID), jStr)   //issue the shutdown command
    return
}

/*! \brief Gets the info about a droplet from its id
 */
func (do DO_c) getDropletFromID (id int) (*do_droplet_t) {
    resp, _ := do.request(fmt.Sprintf("droplets/%d", id), nil)   //get the status
    var m struct {
        Droplet do_droplet_t    `json:"droplet"`
    }
    json.Unmarshal(resp, &m)
    return &m.Droplet
}

/*! \brief Gets the node's info from it's name
 */
func (do DO_c) getDropletFromName (name string) (*do_droplet_t, error) {
    name = strings.ToLower(name)
    page := 1
    perPage := 10
    
    for true {
        resp, err := do.request(fmt.Sprintf("droplets?page=%d&per_page=%d", page, perPage), nil)
        if err == nil {
            var droplets struct {
                Droplets []do_droplet_t    `json:"droplets"`
            }
            err = json.Unmarshal(resp, &droplets)
            
            for _, drop := range(droplets.Droplets) {
                if strings.Compare(strings.ToLower(drop.Name), name) == 0 { //this is our node!
                    return &drop, nil
                }
            }
            
            //didn't find it
            if len(droplets.Droplets) < perPage {   //we don't have any more pages of nodes
                return nil, err
            }
        } else {
            return nil, err //this is bad
        }
        
        page++; //ramp to the next one, we're not done
    }
    return  nil, nil    //won't get here
}

/*! \brief Gets a specific domain record from the domain and sub-domain
 */
func (do DO_c) getDomainRecord (domain, subDomain string) (dr *do_domain_record_t, err error) {
    pages := 1
    //first step is to get a list of current subdomains from this parent domain
    if do.Verbose { fmt.Println("Getting list of current subdomains") }
    for pages > 0 {
        nextUrl := fmt.Sprintf("domains/%s/records?page=%d", domain, pages)    //this is the next url to request
        resp, err := do.request(nextUrl, nil)
        if err == nil {
            var records struct {
                Records []do_domain_record_t    `json:"domain_records"`
                Links   struct {
                    Pages   struct {
                        Next    string  `json:"next"`
                    }   `json:"pages"`
                }   `json:"links"`
            }
            err = json.Unmarshal(resp, &records)
            if err == nil {
                //loop through these records looking for a matched subdomain
                for _, sd := range (records.Records) {
                    if strings.Compare(strings.ToLower(sd.Name), subDomain) == 0 {  //the record exists
                        return &sd, nil  //we found it
                    }
                }
                
                //keep searching as long as we have a "next" url
                if len(records.Links.Pages.Next) > 0 {
                    pages++
                } else {
                    pages = 0   //we're done
                }
            } else {
                return nil, err
            }
        } else {
            return nil, err
        }
    }
    
    //if we're here it's cause it didn't exist yet
    return nil, nil
}

/*! \brief Simple function that waits for a node to be the status we're looking for
 */
func (do DO_c) waitForNodeStatus (id int, status string, maxTries int) bool {
    time.Sleep(time.Second * 3)
    dStatus := do.getDropletFromID(id)
    
    if dStatus.Status == status { return true } //we're good
    maxTries--
    
    if maxTries < 0 { return false }    //this is bad
    return do.waitForNodeStatus(id, status, maxTries)   //recurive call as we wait again
}

  //-------------------------------------------------------------------------------------------------------------------------//
 //----- DOMAIN FUNCTIONS --------------------------------------------------------------------------------------------------//
//-------------------------------------------------------------------------------------------------------------------------//

/*! \brief This will assign a floating ip address to a specific node
 */
func (do DO_c) AssignFloatingIP (ip string, id int) error {
    data := do_t {Type: "assign", ID: id}
    jStr, _ := json.Marshal(data)
    _, err := do.request(fmt.Sprintf("floating_ips/%s/actions", ip), jStr)
    return err
}

/*! \brief Gets the existing information about a floating ip address
 */
func (do DO_c) GetFloatingIP (ip string) (int, error) {
    resp, err := do.request(fmt.Sprintf("floating_ips/%s", ip), nil)
    if err == nil {
        floater := do_floating_t{}
        err = json.Unmarshal(resp, &floater)
        
        return floater.FloatingIP.Droplet.ID, err
    } else {
        return 0, err
    }
}

/*! \brief Handles full logic of creating, updating, or leaving alone a domain record
 */
func (do DO_c) AssignDomainRecord (domain, domainType, subDomain, ip string) error {
    domain = strings.ToLower(domain)
    subDomain = strings.ToLower(subDomain)
    dr, err := do.getDomainRecord(domain, subDomain)    //see if this already exists
    
    if err == nil {
        if dr == nil {  //it doesn't exist yet, so create it
            if do.Verbose { fmt.Println("SubDomain does not exist, creating...") }
            return do.createDomainRecord(domain, domainType, subDomain, ip)
        } else {    //it exists already
            if strings.Compare(domainType, dr.Type) == 0 {
                if do.Verbose { fmt.Println("SubDomain already exists and is correct") }
                return nil  //we're done
            } else {
                if do.Verbose { fmt.Println("SubDomain already exists but needs to be updated") }
                //return do.updateDomainRecord()
                return fmt.Errorf("Fuction not in place yet")
            }
        }
    }
    
    return err
}

/*! \brief Deletes an existing domain record
 */
func (do DO_c) DeleteDomainRecord (domain, subDomain string) error {
    domain = strings.ToLower(domain)
    subDomain = strings.ToLower(subDomain)
    dr, err := do.getDomainRecord(domain, subDomain)    //see if this already exists
    
    if err == nil {
        if dr == nil {  //it doesn't exist, so we're good
            if do.Verbose { fmt.Println("SubDomain does not exist, nothing to do...") }
        } else {    //it exists
            fmt.Println("Deleting SubDomain " + subDomain)
            err = do.deleteRequest(fmt.Sprintf("domains/%s/records/%d", domain, dr.ID))     //delete it
        }
    }
    
    return err
}

  //-------------------------------------------------------------------------------------------------------------------------//
 //----- NODE FUNCTIONS ----------------------------------------------------------------------------------------------------//
//-------------------------------------------------------------------------------------------------------------------------//


/*! \brief Creates a new node, if it doesn't already exist
 */
func (do DO_c) CreateNode (name, region, tag string, size int, image, sshKey string, fileOutput *FileOutput_t) (err error) {
    //see if the droplet already exists
    droplet, err := do.getDropletFromName (name)
    
    if err == nil {
        if droplet == nil {  //we didn't get a droplet back
            if do.Verbose { fmt.Println("Node does not exist, creating...") }
            var node = struct {
                Name    string  `json:"name"`
                Region  string  `json:"region"`
                Size    string  `json:"size"`
                Image   string  `json:"image"`
                Keys    []string    `json:"ssh_keys,omitempty"`
                Tags    []string    `json:"tags,omitempty"`
            }{Name: name, Region: region, Size: fmt.Sprintf("%dgb", size), Image: image}
            
            //see if we have any sshkeys for this
            if len(sshKey) > 0 { node.Keys = append(node.Keys, sshKey) }
            
            //see if we have any tag for this node
            if len(tag) > 0 { node.Tags = append(node.Tags, tag) }
            
            jStr, _ := json.Marshal(node)
            _, err = do.request("droplets", jStr)
            
            if err == nil {
                //we need to give digital ocean a few seconds to assign an ip address
                time.Sleep(5 * time.Second)
                droplet, err = do.getDropletFromName (name) //get the droplet again, we need the ip address
            }
            
            if do.Verbose { fmt.Println("New node created successfully") }
        } else {
            if do.Verbose { fmt.Println("Node by that name already exists") }
        }
        
        if err == nil && droplet != nil { //this worked
            fileOutput.Droplet = *droplet
        }
    }
    
    return
}

/*! \brief This will delete a node
 */
func (do DO_c) DeleteNode (name string) (err error) {
    droplet, err := do.getDropletFromName (name)
    
    if err == nil {
        if droplet != nil {    //we have a droplet we want to remove
            fmt.Println("Deleting node: " + name)
            err = do.deleteRequest(fmt.Sprintf("droplets/%d", droplet.ID))     //delete it
        } else {
            if do.Verbose { fmt.Println("Droplet does not exist, nothing to do...") }
        }
    }
    
    return
}

/*! \brief Resizes the node to the new target size
 *  This needs to power the node off first, then resize it, then start it
 */
func (do DO_c) ResizeNode (name string, size int) (err error) {
    droplet, err := do.getDropletFromName (name)    //get this droplet
    
    if err == nil {
        if droplet != nil {    //we have a droplet we want to remove
            if int(droplet.Memory / 1024) != size {
                fmt.Println("Resizing node: " + name)
                err = do.shutdownNode(droplet)  //first step is to shut it down
                if err == nil {
                    //now we issue the resize
                    simple := do_t{Type: "resize", Size: fmt.Sprintf("%dgb", size)}
                    jStr, _ := json.Marshal(simple)
                    if do.Verbose { fmt.Println("Resizing node '%s' to %dgb", name, size) }
                    _, err = do.request(fmt.Sprintf("droplets/%d/actions", droplet.ID), jStr)   //issue the resize command
                    
                    //this can take a while, so we wait a minute, but we want the node to start as soon as possible
                    if err == nil {
                        locked := true
                        fmt.Println("Waiting for node to finish resize")
                        for locked {
                            time.Sleep(time.Second * 20)    //wait a little while, this takes some time
                            dStatus := do.getDropletFromID(droplet.ID)
                            
                            if !dStatus.Locked {    //we've been waiting for this moment
                                do.startNode(droplet)   //start this node
                                locked = false
                            }
                        }
                        
                        //now we just wait for the node to be active
                        do.waitForNodeStatus(droplet.ID, "active", 10)
                    }
                }
            } else {
                if do.Verbose { fmt.Println("Droplet already the target size.  Skipping") }
            }
        } else {
            if do.Verbose { fmt.Println("Droplet does not exist, please check the name") }
        }
    }
    
    return
}

