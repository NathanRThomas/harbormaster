/*! \file cf.go
    \brief Our generic class for a wrapper around the cloud flare stuff
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

const cf_base_url          = "https://api.cloudflare.com/client/v4/zones"

//-------------------------------------------------------------------------------------------------------------------------//
//----- STRUCTS -----------------------------------------------------------------------------------------------------------//
//-------------------------------------------------------------------------------------------------------------------------//

type CF_config_t struct {
    APIKey  string  `json:"api_key"`
    Email   string  `json:"email"`
    Zone    string  `json:"zone"`
}

type CF_c struct {
    Verbose, SuperVerbose     bool
    Config      CF_config_t
}

//-------------------------------------------------------------------------------------------------------------------------//
//----- PRIVATE FUNCTIONS -------------------------------------------------------------------------------------------------//
//-------------------------------------------------------------------------------------------------------------------------//

func (cf CF_c) request (url string, jStr []byte, put []byte) (body []byte, err error) {
    var req *http.Request
    
    if len(jStr) > 0 {    //we're posting data
        finalUrl := fmt.Sprintf("%s/%s/%s", cf_base_url, cf.Config.Zone, url)
        cf.superMessage("url: " + finalUrl)
        req, err = http.NewRequest("POST", finalUrl, bytes.NewBuffer(jStr))
    } else if len(put) > 0 {  //put request
        finalUrl := fmt.Sprintf("%s/%s/%s", cf_base_url, cf.Config.Zone, url)
        cf.superMessage("url: " + finalUrl)
        req, err = http.NewRequest("PUT", finalUrl, bytes.NewBuffer(put))
    } else {    //we're doing a get
        finalUrl := fmt.Sprintf("%s/%s/%s", cf_base_url, cf.Config.Zone, url)
        cf.superMessage("url: " + finalUrl)
        req, err = http.NewRequest("GET", finalUrl, nil)
    }
    
    if err == nil {
        req.Header.Set("Content-Type", "application/json")
        req.Header.Set("X-Auth-Email", cf.Config.Email)
        req.Header.Set("X-Auth-Key", cf.Config.APIKey)
        
        client := &http.Client{}
        resp, err := client.Do(req)
        if err == nil {
            defer resp.Body.Close()
            body, _ = ioutil.ReadAll(resp.Body)
            
            if cf.SuperVerbose {
                fmt.Println("response Status:", resp.Status)
                fmt.Println("response Headers:", resp.Header)
                fmt.Println("response Body:", string(body[:]))
            }
            
            if resp.StatusCode >= 300 {
                return nil, fmt.Errorf("Response code: %s", resp.Status)
            }
        } else {
            return nil, err
        }
    }
    
    return
}

/*! \brief For when we do a delete request where we aren't expecting a body, only a return code
 */
func (cf CF_c) deleteRequest (url string) (err error) {
    req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/%s/%s", cf_base_url, cf.Config.Zone, url), nil)
    
    if err == nil {
        req.Header.Set("Content-Type", "application/json")
        req.Header.Set("X-Auth-Email", cf.Config.Email)
        req.Header.Set("X-Auth-Key", cf.Config.APIKey)
        
        client := &http.Client{}
        resp, err := client.Do(req)
        if err == nil {
            defer resp.Body.Close()
            
            if cf.SuperVerbose {
                fmt.Println("response Status:", resp.Status)
                fmt.Println("response Headers:", resp.Header)
            }
            
            if resp.StatusCode >= 300 {
                return fmt.Errorf("Response code: %s", resp.Status)
            }
        } else {
            return err
        }
    }
    
    return
}

func (cf CF_c) verboseMessage (msg string) {
    if cf.Verbose { fmt.Println(msg) }
}

func (cf CF_c) superMessage (msg string) {
    if cf.SuperVerbose { fmt.Println(msg) }
}

/*! \brief Creates a domain record when one doesn't exist yet
 */
func (cf CF_c) createDomainRecord (domainType, subDomain, ip string) (err error) {
    record := struct { Type    string  `json:"type"`
        Name    string  `json:"name"`
        Content string  `json:"content"`
    }{domainType, subDomain, ip}
    
    jStr, _ := json.Marshal(record)
    _, err = cf.request("dns_records", jStr, nil)
    return
}

/*! \brief Updates an existing domain record
 */
func (cf CF_c) updateDomainRecord (id, domainType, subDomain, ip string) (err error) {
    record := struct { Type    string  `json:"type"`
        Name    string  `json:"name"`
        Content string  `json:"content"`
    }{domainType, subDomain, ip}
    
    jStr, _ := json.Marshal(record)
    _, err = cf.request("dns_records/" + id, nil, jStr)
    return
}

/*! \brief Gets a specific domain record from the domain and sub-domain
 */
func (cf CF_c) getDomainRecord (subDomain string) (string, error) {
    var err error
    pages := 1
    //first step is to get a list of current subdomains from this parent domain
    cf.verboseMessage("Getting list of current subdomains")
    for pages > 0 {
        nextUrl := fmt.Sprintf("dns_records?page=%d", pages)    //this is the next url to request
        resp, err := cf.request(nextUrl, nil, nil)
        if err == nil {
            var records struct {
                Success bool    `json:"success"`
                ResultInfo  struct {
                    TotalPages  int     `json:"total_pages"`
                }   `json:"result_info"`
                
                Records  []struct {
                    ID      string  `json:"id"`
                    Name    string  `json:"name"`
                    ZoneName    string  `json:"zone_name"`
                }   `json:"result"`
            }
            
            err = json.Unmarshal(resp, &records)
            if err == nil {
                //loop through these records looking for a matched subdomain
                for _, sd := range (records.Records) {
                    if strings.Compare(strings.ToLower(sd.Name), fmt.Sprintf("%s.%s", subDomain, sd.ZoneName)) == 0 {  //the record exists
                        return sd.ID, nil  //we found it
                    }
                }
                
                //keep searching as long as we have a "next" page
                if records.ResultInfo.TotalPages > pages {
                    pages++
                } else {
                    pages = 0   //we're done
                }
            } else {
                return "", err
            }
        } else {
            return "", err
        }
    }
    
    //if we're here it's cause it didn't exist yet
    return "", err
}

  //-------------------------------------------------------------------------------------------------------------------------//
 //----- DOMAIN FUNCTIONS --------------------------------------------------------------------------------------------------//
//-------------------------------------------------------------------------------------------------------------------------//

/*! \brief Handles full logic of creating, updating, or leaving alone a domain record
 */
func (cf CF_c) AssignDomainRecord (domainType, subDomain, ip string) error {
    subDomain = strings.ToLower(subDomain)
    id, err := cf.getDomainRecord(subDomain)    //see if this already exists
    
    if err == nil {
        if len(id) == 0 {  //it doesn't exist yet, so create it
            cf.verboseMessage("SubDomain does not exist, creating...")
            return cf.createDomainRecord(domainType, subDomain, ip)
        } else {    //it exists already
            cf.verboseMessage("SubDomain already exists, updating")
            return cf.updateDomainRecord(id, domainType, subDomain, ip)
        }
    }
    
    return err
}

/*! \brief Deletes an existing domain record
 */
func (cf CF_c) DeleteDomainRecord (subDomain string) error {
    subDomain = strings.ToLower(subDomain)
    id, err := cf.getDomainRecord(subDomain)    //see if this already exists
    
    if err == nil {
        if len(id) == 0 {  //it doesn't exist, so we're good
            cf.verboseMessage("SubDomain does not exist, nothing to do...")
        } else {    //it exists
            cf.verboseMessage("Deleting SubDomain " + subDomain)
            err = cf.deleteRequest("dns_records/" + id)     //delete it
        }
    }
    
    return err
}

