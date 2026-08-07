package tests

import (
	"fmt"
	"io"
	"net/http"
	"strings"
)

func subTestMetrics(g *testGroup) {
	g.regSubTest("basic", metrics_basic_test)
}

func downloadURLWithStatusCode(u string) (int, string, error) {
	resp, err := http.Get(u)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, "", err
	}
	return resp.StatusCode, string(body), nil
}

func metrics_basic_test(mc *mockServer) error {

	maddr := fmt.Sprintf("http://127.0.0.1:%d/", mc.metricsPort())

	mc.Do("SET", "metrics_test_1", "1", "FIELD", "foo", 5.5, "POINT", 5, 5)
	mc.Do("SET", "metrics_test_2", "2", "FIELD", "foo", 19.19, "POINT", 19, 19)
	mc.Do("SET", "metrics_test_2", "3", "FIELD", "foo", 19.19, "POINT", 19, 19)
	mc.Do("SET", "metrics_test_2", "truck1:driver", "STRING", "John Denton")

	status, index, err := downloadURLWithStatusCode(maddr)
	if err != nil {
		return err
	}
	if status != 200 {
		return fmt.Errorf("Expected status code 200, got: %d", status)
	}
	if !strings.Contains(index, "<a href") {
		return fmt.Errorf("missing link on index page")
	}

	status, metrics, err := downloadURLWithStatusCode(maddr + "metrics")
	if err != nil {
		return err
	}
	if status != 200 {
		return fmt.Errorf("Expected status code 200, got: %d", status)
	}
	for _, want := range []string{
		`tile38_connected_clients`,
		`tile38_cmd_duration_seconds_count{cmd="set"}`,
		`go_build_info`,
		`go_threads`,
		`tile38_collection_objects{col="metrics_test_1"} 1`,
		`tile38_collection_objects{col="metrics_test_2"} 3`,
		`tile38_collection_points{col="metrics_test_2"} 2`,
		`tile38_replication_info`,
		`role="leader"`,
	} {
		if !strings.Contains(metrics, want) {
			return fmt.Errorf("wanted metric: %s, got: %s", want, metrics)
		}
	}
	return nil
}
