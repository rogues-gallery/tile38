package tests

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"

	"github.com/gomodule/redigo/redis"
	"github.com/tidwall/pretty"
	"github.com/tidwall/sjson"
)

func fence_roaming_webhook_test(mc *mockServer) error {
	car1, car2, expected := roamingTestData()
	finalErr := make(chan error)

	// Create a connection for subscribing to geofence notifications
	sc, err := redis.Dial("tcp", fmt.Sprintf(":%d", mc.port))
	if err != nil {
		return err
	}
	defer sc.Close()

	actual := []string{}
	// Create the test http server that will capture all messages
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := func() error {
			// Read the request body
			body, err := ioutil.ReadAll(r.Body)
			if err != nil {
				return err
			}

			// If the new message doesn't match whats expected an error
			// should be returned
			actual = append(actual, cleanMessage(body))
			pos := len(actual) - 1
			if len(expected) < pos+1 {
				return fmt.Errorf("More messages than expected were received : '%s'", actual[pos])
			}
			if actual[pos] != expected[pos] {
				return fmt.Errorf("Expected '%s' but got '%s'", expected[pos],
					actual[pos])
			}
			if len(actual) == len(expected) {
				finalErr <- nil
			}
			return nil
		}(); err != nil {
			finalErr <- err
		}
		fmt.Fprintln(w, "OK!")
	}))
	defer ts.Close()

	_, err = sc.Do("SETHOOK", "carshook", ts.URL, "NEARBY", "cars", "FENCE", "ROAM", "cars", "*", 1000)
	if err != nil {
		return err
	}

	// Create the base connection for setting up points and geofences
	bc, err := redis.Dial("tcp", fmt.Sprintf(":%d", mc.port))
	if err != nil {
		return err
	}
	defer bc.Close()

	// Fire all car movement commands on the base client
	for i := range car1 {
		if _, err := bc.Do("SET", "cars", "car1", "POINT", car1[i][1],
			car1[i][0]); err != nil {
			return err
		}
		if _, err := bc.Do("SET", "cars", "car2", "POINT", car2[i][1],
			car2[i][0]); err != nil {
			return err
		}
	}

	return <-finalErr
}

func goMultiFunc(mc *mockServer, fns ...func() error) error {
	errs := make([]error, len(fns))
	var wg sync.WaitGroup
	wg.Add(len(fns))
	for i := 0; i < len(fns); i++ {
		go func(i int) {
			defer wg.Done()
			errs[i] = fns[i]()
		}(i)
	}
	wg.Wait()
	var ferrs []error
	for i := 0; i < len(errs); i++ {
		if errs[i] != nil {
			ferrs = append(ferrs, errs[i])
		}
	}
	if len(ferrs) == 0 {
		return nil
	}
	if len(ferrs) == 1 {
		return ferrs[0]
	}
	return fmt.Errorf("%v", ferrs)
}

func fence_roaming_live_test(mc *mockServer) error {
	car1, car2, expected := roamingTestData()
	var liveReady sync.WaitGroup
	liveReady.Add(1)
	return goMultiFunc(mc,
		func() error {
			sc, err := redis.DialTimeout("tcp", fmt.Sprintf(":%d", mc.port),
				0, time.Second*5, time.Second*5)
			if err != nil {
				liveReady.Done()
				return err
			}
			defer sc.Close()
			// Set up a live geofence stream
			reply, err := redis.String(
				sc.Do("NEARBY", "cars", "FENCE", "ROAM", "cars", "*", 1000),
			)
			if err != nil {
				liveReady.Done()
				return err
			}
			if reply != "OK" {
				liveReady.Done()
				return fmt.Errorf("expected 'OK', got '%v'", reply)
			}
			liveReady.Done()
			for i := 0; i < len(expected); i++ {
				reply, err := redis.String(sc.Receive())
				if err != nil {
					return err
				}
				reply = cleanMessage([]byte(reply))
				if reply != expected[i] {
					return fmt.Errorf("Expected '%s' but got '%s'",
						expected[i], reply)
				}
			}
			return nil
		},
		func() error {
			liveReady.Wait()
			bc, err := redis.Dial("tcp", fmt.Sprintf(":%d", mc.port))
			if err != nil {
				return err
			}
			defer bc.Close()

			// Fire all car movement commands on the base client
			for i := range car1 {

				if _, err := bc.Do("SET", "cars", "car1", "POINT", car1[i][1],
					car1[i][0]); err != nil {
					return err
				}
				if _, err := bc.Do("SET", "cars", "car2", "POINT", car2[i][1],
					car2[i][0]); err != nil {
					return err
				}
			}

			return nil
		},
	)
}

func fence_roaming_channel_test(mc *mockServer) error {
	car1, car2, expected := roamingTestData()
	finalErr := make(chan error)

	go func() {
		// Create a connection for subscribing to geofence notifications
		sc, err := redis.Dial("tcp", fmt.Sprintf(":%d", mc.port))
		if err != nil {
			finalErr <- err
			return
		}
		defer sc.Close()

		if _, err := sc.Do("SETCHAN", "carschan", "NEARBY", "cars", "FENCE", "ROAM", "cars", "*", 1000); err != nil {
			finalErr <- err
			return
		}

		// Subscribe the subscription client to the * pattern
		psc := redis.PubSubConn{Conn: sc}
		if err := psc.PSubscribe("carschan"); err != nil {
			finalErr <- err
			return
		}

		actual := []string{}
		for sc.Err() == nil {
			if err := func() error {
				var body []byte
				switch v := psc.Receive().(type) {
				case redis.Message:
					body = v.Data
				case error:
					return err
				}
				if len(body) == 0 {
					return nil
				}

				// If the new message doesn't match whats expected an error
				// should be returned
				actual = append(actual, cleanMessage(body))
				pos := len(actual) - 1
				if len(expected) < pos+1 {
					return fmt.Errorf("More messages than expected were received : '%s'", actual[pos])
				}
				if actual[pos] != expected[pos] {
					return fmt.Errorf("Expected '%s' but got '%s'", expected[pos],
						actual[pos])
				}
				if len(actual) == len(expected) {
					finalErr <- nil
				}
				return nil
			}(); err != nil {
				finalErr <- err
			}
		}
	}()

	// Create the base connection for setting up points and geofences
	bc, err := redis.Dial("tcp", fmt.Sprintf(":%d", mc.port))
	if err != nil {
		return err
	}
	defer bc.Close()

	// Fire all car movement commands on the base client
	for i := range car1 {
		if _, err := bc.Do("SET", "cars", "car1", "POINT", car1[i][1],
			car1[i][0]); err != nil {
			return err
		}
		if _, err := bc.Do("SET", "cars", "car2", "POINT", car2[i][1],
			car2[i][0]); err != nil {
			return err
		}
	}

	return <-finalErr
}

func cleanMessage(body []byte) string {
	// Remove fields that are non-deterministic or use case specific
	msg, _ := sjson.Delete(string(body), "group")
	msg, _ = sjson.Delete(msg, "time")
	msg, _ = sjson.Delete(msg, "hook")
	msg = string(pretty.Ugly([]byte(msg)))
	return msg
}

func roamingTestData() (car1 [][]float64, car2 [][]float64, output []string) {
	car1 = [][]float64{
		{-111.93669319152832, 33.414750027566235},
		{-111.93051338195801, 33.414750027566235},
		{-111.92416191101074, 33.414750027566235},
		{-111.91789627075195, 33.414750027566235},
		{-111.9111156463623, 33.414750027566235},
		{-111.90510749816895, 33.414750027566235},
		{-111.89746856689453, 33.414750027566235},
	}
	car2 = [][]float64{
		{-111.89746856689453, 33.414750027566235},
		{-111.90519332885742, 33.414750027566235},
		{-111.91154479980467, 33.414750027566235},
		{-111.91781044006346, 33.414750027566235},
		{-111.92416191101074, 33.414750027566235},
		{-111.93059921264648, 33.414750027566235},
		{-111.93660736083984, 33.414750027566235},
	}
	output = []string{
		`{"command":"set","detect":"roam","key":"cars","id":"car1","object":{"type":"Point","coordinates":[-111.91789627075195,33.414750027566235]},"nearby":{"key":"cars","id":"car2","object":{"type":"Point","coordinates":[-111.91154479980467,33.414750027566235]},"meters":589.512}}`,
		`{"command":"set","detect":"roam","key":"cars","id":"car2","object":{"type":"Point","coordinates":[-111.91781044006346,33.414750027566235]},"nearby":{"key":"cars","id":"car1","object":{"type":"Point","coordinates":[-111.91789627075195,33.414750027566235]},"meters":7.966}}`,
		`{"command":"set","detect":"roam","key":"cars","id":"car1","object":{"type":"Point","coordinates":[-111.9111156463623,33.414750027566235]},"nearby":{"key":"cars","id":"car2","object":{"type":"Point","coordinates":[-111.91781044006346,33.414750027566235]},"meters":621.377}}`,
		`{"command":"set","detect":"roam","key":"cars","id":"car2","object":{"type":"Point","coordinates":[-111.92416191101074,33.414750027566235]},"faraway":{"key":"cars","id":"car1","object":{"type":"Point","coordinates":[-111.9111156463623,33.414750027566235]},"meters":1210.89}}`,
	}
	return
}
