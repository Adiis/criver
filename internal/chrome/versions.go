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

// VersionData holds both the top 3 versions and the full list for searching.
type VersionData struct {
	Top []string // latest patch per top 3 major versions
	All []string // every known version, sorted descending
}

// FetchVersions fetches known-good-versions.json and returns both the top 3
// major versions and the full version list for searching.
func FetchVersions() (VersionData, error) {
	resp, err := http.Get("https://googlechromelabs.github.io/chrome-for-testing/known-good-versions.json")
	if err != nil {
		return VersionData{}, fmt.Errorf("fetching versions: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return VersionData{}, fmt.Errorf("unexpected status: %s", resp.Status)
	}

	var data knownGoodVersions
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return VersionData{}, fmt.Errorf("decoding versions: %w", err)
	}

	// Collect all versions sorted descending.
	all := make([]string, len(data.Versions))
	for i, v := range data.Versions {
		all[i] = v.Version
	}
	sort.Slice(all, func(i, j int) bool {
		return versionLess(all[j], all[i]) // descending
	})

	// Top 3 major versions.
	latest := make(map[int]string)
	for _, v := range all {
		major := ParseMajor(v)
		if cur, ok := latest[major]; !ok || versionLess(cur, v) {
			latest[major] = v
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

	top := make([]string, count)
	for i := 0; i < count; i++ {
		top[i] = latest[majors[i]]
	}

	return VersionData{Top: top, All: all}, nil
}

// SearchVersions returns versions from the full list that contain the query,
// limited to maxResults.
func SearchVersions(all []string, query string, maxResults int) []string {
	var matches []string
	for _, v := range all {
		if strings.Contains(v, query) {
			matches = append(matches, v)
			if len(matches) >= maxResults {
				break
			}
		}
	}
	return matches
}
