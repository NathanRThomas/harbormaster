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
                fmt.Println("response Body:", string(body[:100]))
            }
        } else {
            return nil, err
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

//-------------------------------------------------------------------------------------------------------------------------//
//----- PUBLIC FUNCTIONS --------------------------------------------------------------------------------------------------//
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
    
    //first step is to get a list of current subdomains from this parent domain
    if do.Verbose { fmt.Println("Getting list of current subdomains") }
    resp, err := do.request(fmt.Sprintf("domains/%s/records", domain), nil)
    if err == nil {
        var records struct {
            Records []do_domain_record_t    `json:"domain_records"`
        }
        err = json.Unmarshal(resp, &records)
        
        if err == nil {
            //loop through these records looking for a matched subdomain
            for _, sd := range (records.Records) {
                if strings.Compare(strings.ToLower(sd.Name), subDomain) == 0 {  //the record exists
                    if strings.Compare(domainType, sd.Type) == 0 {
                        if do.Verbose { fmt.Println("SubDomain already exists and is correct") }
                        return nil  //we're done
                    } else {
                        if do.Verbose { fmt.Println("SubDomain already exists but needs to be updated") }
                        //return do.updateDomainRecord()
                        return fmt.Errorf("Fuction not in place yet")
                    }
                }
            }
            
            //if we're here it's cause it didn't exist yet
            if do.Verbose { fmt.Println("SubDomain does not exist, creating...") }
            return do.createDomainRecord(domain, domainType, subDomain, ip)
        }
    }
    
    return err
}

//curl -X POST -H "Content-Type: application/json" -H "Authorization: Bearer b7d03a6947b217efb6f3ec3bd3504582" 
//-d '{"type":"assign","droplet_id":8219222}' "https://api.digitalocean.com/v2/floating_ips/45.55.96.47/actions" 