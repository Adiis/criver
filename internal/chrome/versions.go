package chrome

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

type knownGoodVersions struct {
	Versions []versionEntry `json:"versions"`
}

type versionEntry struct {
	Version string `json:"version"`
}

// ParseMajor extracts the major version number from "X.Y.Z.W".
func ParseMajor(version string) int {
	parts := strings.SplitN(version, ".", 2)
	n, _ := strconv.Atoi(parts[0])
	return n
}

// versionLess returns true if a < b using numeric comparison of each segment.
func versionLess(a, b string) bool {
	pa := strings.Split(a, ".")
	pb := strings.Split(b, ".")
	for i := 0; i < len(pa) && i < len(pb); i++ {
		na, _ := strconv.Atoi(pa[i])
		nb, _ := strconv.Atoi(pb[i])
		if na != nb {
			return na < nb
		}
	}
	return len(pa) < len(pb)
}

// FetchTopVersions fetches known-good-versions.json and returns the latest
// patch for each of the top 3 major versions, sorted descending.
func FetchTopVersions() ([]string, error) {
	resp, err := http.Get("https://googlechromelabs.github.io/chrome-for-testing/known-good-versions.json")
	if err != nil {
		return nil, fmt.Errorf("fetching versions: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %s", resp.Status)
	}

	var data knownGoodVersions
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("decoding versions: %w", err)
	}

	latest := make(map[int]string)
	for _, v := range data.Versions {
		major := ParseMajor(v.Version)
		if cur, ok := latest[major]; !ok || versionLess(cur, v.Version) {
			latest[major] = v.Version
		}
	}

	majors := make([]int, 0, len(latest))
	for m := range latest {
		majors = append(majors, m)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(majors)))

	count := 3
	if len(majors) < count {
		count = len(majors)
	}

	result := make([]string, count)
	for i := 0; i < count; i++ {
		result[i] = latest[majors[i]]
	}
	return result, nil
}
