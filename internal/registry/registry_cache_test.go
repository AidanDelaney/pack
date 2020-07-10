package registry

import (
	"bytes"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing/object"

	ilogging "github.com/buildpacks/pack/internal/logging"
	"github.com/buildpacks/pack/logging"
	h "github.com/buildpacks/pack/testhelpers"
)

func TestRegistryCache(t *testing.T) {
	spec.Run(t, "RegistryCache", testRegistryCache, spec.Parallel(), spec.Report(report.Terminal{}))
}

func testRegistryCache(t *testing.T, when spec.G, it spec.S) {
	var (
		tmpDir          string
		err             error
		registryFixture string
		outBuf          bytes.Buffer
		logger          logging.Logger
	)

	it.Before(func() {
		logger = ilogging.NewLogWithWriters(&outBuf, &outBuf)

		tmpDir, err = ioutil.TempDir("", "registry")
		h.AssertNil(t, err)

		registryFixture = h.CreateRegistryFixture(t, tmpDir, filepath.Join("..", "..", "testdata", "registry"))
	})

	it.After(func() {
		err := os.RemoveAll(tmpDir)
		h.AssertNil(t, err)
	})

	when("#NewDefaultRegistryCache", func() {
		it("creates a RegistryCache with default URL", func() {
			registryCache, err := NewDefaultRegistryCache(logger, tmpDir)
			h.AssertNil(t, err)
			normalizedURL, err := url.Parse("https://github.com/buildpacks/registry-index")
			h.AssertNil(t, err)

			h.AssertEq(t, registryCache.url, normalizedURL)
		})
	})

	when("#NewRegistryCache", func() {
		when("home doesn't exist", func() {
			it("fails to create a registry cache", func() {
				_, err := NewRegistryCache(logger, "/tmp/not-exist", "not-here")
				h.AssertError(t, err, "finding home")
			})
		})

		when("registryURL isn't a valid url", func() {
			it("fails to create a registry cache", func() {
				_, err := NewRegistryCache(logger, tmpDir, "://bad-uri")
				h.AssertError(t, err, "parsing registry url")
			})
		})

		it("creates a RegistryCache", func() {
			registryCache, err := NewRegistryCache(logger, tmpDir, registryFixture)
			h.AssertNil(t, err)
			expectedRoot := filepath.Join(tmpDir, "registry")
			actualRoot := strings.Split(registryCache.Root, "-")[0]
			h.AssertEq(t, actualRoot, expectedRoot)
		})
	})

	when("#LocateBuildpack", func() {
		var (
			registryCache Cache
		)

		it.Before(func() {
			registryCache, err = NewRegistryCache(logger, tmpDir, registryFixture)
			h.AssertNil(t, err)
		})

		it("locates a buildpack without version", func() {
			bp, err := registryCache.LocateBuildpack("example/java")
			h.AssertNil(t, err)
			h.AssertNotNil(t, bp)

			h.AssertEq(t, bp.Namespace, "example")
			h.AssertEq(t, bp.Name, "java")
			h.AssertEq(t, bp.Version, "1.0.0")
		})

		it("locates a buildpack without version", func() {
			bp, err := registryCache.LocateBuildpack("example/foo")
			h.AssertNil(t, err)
			h.AssertNotNil(t, bp)

			h.AssertEq(t, bp.Namespace, "example")
			h.AssertEq(t, bp.Name, "foo")
			h.AssertEq(t, bp.Version, "1.2.0")
		})

		it("locates a buildpack with version", func() {
			bp, err := registryCache.LocateBuildpack("example/foo@1.1.0")
			h.AssertNil(t, err)
			h.AssertNotNil(t, bp)

			h.AssertEq(t, bp.Namespace, "example")
			h.AssertEq(t, bp.Name, "foo")
			h.AssertEq(t, bp.Version, "1.1.0")
		})

		it("returns error if can't parse buildpack id", func() {
			_, err := registryCache.LocateBuildpack("quack")
			h.AssertError(t, err, "parsing buildpacks registry id")
		})

		it("returns error if buildpack id is empty", func() {
			_, err := registryCache.LocateBuildpack("example/")
			h.AssertError(t, err, "empty buildpack name")
		})

		it("returns error if can't find buildpack with requested id", func() {
			_, err := registryCache.LocateBuildpack("example/qu")
			h.AssertError(t, err, "reading entry")
		})

		it("returns error if can't find buildpack with requested version", func() {
			_, err := registryCache.LocateBuildpack("example/foo@3.5.6")
			h.AssertError(t, err, "could not find version")
		})
	})

	when("#Refresh", func() {
		var (
			registryCache Cache
		)

		it.Before(func() {
			registryCache, err = NewRegistryCache(logger, tmpDir, registryFixture)
			h.AssertNil(t, err)
		})

		when("registry has new commits", func() {
			it("pulls the latest index", func() {
				h.AssertNil(t, registryCache.Refresh())
				h.AssertGitHeadEq(t, registryFixture, registryCache.Root)

				r, err := git.PlainOpen(registryFixture)
				h.AssertNil(t, err)

				w, err := r.Worktree()
				h.AssertNil(t, err)

				commit, err := w.Commit("second", &git.CommitOptions{
					Author: &object.Signature{
						Name:  "John Doe",
						Email: "john@doe.org",
						When:  time.Now(),
					},
				})
				h.AssertNil(t, err)

				_, err = r.CommitObject(commit)
				h.AssertNil(t, err)

				h.AssertNil(t, registryCache.Refresh())
				h.AssertGitHeadEq(t, registryFixture, registryCache.Root)
			})
		})

		when("Root is an empty string", func() {
			it("fails to refresh", func() {
				registryCache.Root = ""
				err = registryCache.Refresh()
				h.AssertError(t, err, "initializing")
			})
		})
	})

	when("#Initialize", func() {
		var (
			registryCache Cache
		)

		it.Before(func() {
			registryCache, err = NewRegistryCache(logger, tmpDir, registryFixture)
			h.AssertNil(t, err)
		})

		when("root is empty string", func() {
			it.Before(func() {
				registryCache.Root = ""
			})

			it("fails to create registry cache", func() {
				err = registryCache.Initialize()
				h.AssertError(t, err, "creating registry cache")
			})

			when("url is empty string", func() {
				it("fails to clone cache", func() {
					registryCache.url = &url.URL{}

					err = registryCache.Initialize()
					h.AssertError(t, err, "cloning remote registry")
				})
			})
		})
	})
}
