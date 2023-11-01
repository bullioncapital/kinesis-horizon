package horizonclient

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/stellar/go/support/clock"
	"github.com/stellar/go/support/errors"
)

// decodeResponse decodes the response from a request to a horizon server
func decodeResponse(resp *http.Response, object interface{}, horizonUrl string, clock *clock.Clock) (err error) {
	defer resp.Body.Close()
	if object == nil {
		// Nothing to decode
		return nil
	}
	decoder := json.NewDecoder(resp.Body)

	u, err := url.Parse(horizonUrl)
	if err != nil {
		return errors.Errorf("unable to parse the provided horizon url: %s", horizonUrl)
	}
	setCurrentServerTime(u.Hostname(), resp.Header["Date"], clock)

	if !(resp.StatusCode >= 200 && resp.StatusCode < 300) {
		horizonError := &Error{
			Response: resp,
		}
		decodeError := decoder.Decode(&horizonError.Problem)
		if decodeError != nil {
			return errors.Wrap(decodeError, "error decoding horizon.Problem")
		}
		return horizonError
	}
	err = decoder.Decode(&object)
	if err != nil {
		return errors.Wrap(err, "error decoding response")
	}
	return
}

// countParams counts the number of parameters provided
func countParams(params ...interface{}) int {
	counter := 0
	for _, param := range params {
		switch param := param.(type) {
		case string:
			if param != "" {
				counter++
			}
		case int:
			if param > 0 {
				counter++
			}
		case uint:
			if param > 0 {
				counter++
			}
		case bool:
			counter++
		default:
			panic("Unknown parameter type")
		}

	}
	return counter
}

// addQueryParams sets query parameters for a url
func addQueryParams(params ...interface{}) string {
	query := url.Values{}

	for _, param := range params {
		switch param := param.(type) {
		case cursor:
			if param != "" {
				query.Add("cursor", string(param))
			}
		case Order:
			if param != "" {
				query.Add("order", string(param))
			}
		case limit:
			if param != 0 {
				query.Add("limit", strconv.Itoa(int(param)))
			}
		case assetCode:
			if param != "" {
				query.Add("asset_code", string(param))
			}
		case assetIssuer:
			if param != "" {
				query.Add("asset_issuer", string(param))
			}
		case includeFailed:
			if param {
				query.Add("include_failed", "true")
			}
		case join:
			if param != "" {
				query.Add("join", string(param))
			}
		case reserves:
			if len(param) > 0 {
				query.Add("reserves", strings.Join(param, ","))
			}
		case map[string]string:
			for key, value := range param {
				if value != "" {
					query.Add(key, value)
				}
			}
		default:
			panic("Unknown parameter type")
		}
	}

	return query.Encode()
}

// setCurrentServerTime saves the current time returned by a horizon server
func setCurrentServerTime(host string, serverDate []string, clock *clock.Clock) {
	if len(serverDate) == 0 {
		return
	}
	st, err := time.Parse(time.RFC1123, serverDate[0])
	if err != nil {
		return
	}
	serverTimeMapMutex.Lock()
	ServerTimeMap[host] = ServerTimeRecord{ServerTime: st.UTC().Unix(), LocalTimeRecorded: clock.Now().UTC().Unix()}
	serverTimeMapMutex.Unlock()
}

// currentServerTime returns the current server time for a given horizon server
func currentServerTime(host string, currentTimeUTC int64) int64 {
	serverTimeMapMutex.Lock()
	st, has := ServerTimeMap[host]
	serverTimeMapMutex.Unlock()
	if !has {
		return 0
	}

	// if it has been more than 5 minutes from the last time, then return 0 because the saved
	// server time is behind.
	if currentTimeUTC-st.LocalTimeRecorded > 60*5 {
		return 0
	}
	return currentTimeUTC - st.LocalTimeRecorded + st.ServerTime
}
