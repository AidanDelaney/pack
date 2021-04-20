package asset_test

import (
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"

	"github.com/buildpacks/pack/internal/asset"
	blob2 "github.com/buildpacks/pack/internal/blob"
	"github.com/buildpacks/pack/internal/dist"
	"github.com/buildpacks/pack/internal/oci"
	h "github.com/buildpacks/pack/testhelpers"
)

func TestLocalFileFetcher(t *testing.T) {
	spec.Run(t, "PackageFileFetcher", testLocalFileFetcher, spec.Parallel(), spec.Report(report.Terminal{}))
}

func testLocalFileFetcher(t *testing.T, when spec.G, it spec.S) {
	var (
		subject              asset.PackageFileFetcher
		assert               = h.NewAssertionManager(t)
		expectedAssetPackage *oci.LayoutPackage
		tmpFile              *os.File
	)
	it.Before(func() {
		var err error
		subject = asset.NewPackageFileFetcher()

		testFile := filepath.Join("testdata", "fake-asset-package.tar")
		testfd, err := os.Open(testFile)
		assert.Nil(err)

		expectedAssetPackage, err = oci.NewLayoutPackage(blob2.NewBlob(
			filepath.Join("testdata", "fake-asset-package.tar"), blob2.RawOption),
		)
		assert.Nil(err)

		tmpFile, err = ioutil.TempFile("", "test-local-file-fetcher-abs")
		assert.Nil(err)

		_, err = io.Copy(tmpFile, testfd)
		assert.Nil(err)
	})
	it.After(func() {
		os.Remove(tmpFile.Name())
	})
	when("using an absolute path", func() {
		it("fetches asset at absolute path", func() {
			ociAssets, err := subject.FetchFileAssets(context.Background(), "/invalid-dir/:::", tmpFile.Name())
			assert.Nil(err)

			assert.Equal(len(ociAssets), 1)

			assertSameAssetLayers(t, ociAssets[0], expectedAssetPackage)
		})
	})
	when("using a local path", func() {
		it("fetches asset relative to 'workingDir'", func() {
			dir := filepath.Dir(tmpFile.Name())
			fileName := filepath.Base(tmpFile.Name())
			ociAssets, err := subject.FetchFileAssets(context.Background(), dir, fileName)
			assert.Nil(err)

			assert.Equal(len(ociAssets), 1)

			assertSameAssetLayers(t, ociAssets[0], expectedAssetPackage)
		})
	})

	when("Failure cases", func() {
		when("fetching a file that does not exist", func() {
			it("errors with helpful message", func() {
				impossibleFileName := "::::"
				_, err := subject.FetchFileAssets(context.Background(), "", impossibleFileName)
				assert.ErrorContains(err, `unable to fetch file asset "::::"`)
			})
		})
		when("local file is not in OCI layout format", func() {
			it.Before(func() {
				var err error
				tmpFile, err = ioutil.TempFile("", "test-local-file-fetcher-not-oci")
				assert.Nil(err)

				_, err = tmpFile.Write([]byte("some random contents"))
				assert.Nil(err)

				_, err = tmpFile.Seek(0, 0)
				assert.Nil(err)
			})
			it("errors with a helpful message", func() {
				_, err := subject.FetchFileAssets(context.Background(), "", tmpFile.Name())
				assert.ErrorContains(err, "unable to read asset as OCI blob")
			})
		})
	})
}

func assertSameAssetLayers(t *testing.T, actual, expected asset.Readable) {
	t.Helper()

	expectedAssetLabel, err := expected.Label("io.buildpacks.asset.layers")
	h.AssertNil(t, err)

	expectedAssetMap := dist.AssetMap{}
	h.AssertNil(t, json.Unmarshal([]byte(expectedAssetLabel), &expectedAssetMap))

	actualAssetLabel, err := actual.Label("io.buildpacks.asset.layers")
	h.AssertNil(t, err)

	actualAssetMap := dist.AssetMap{}
	h.AssertNil(t, json.Unmarshal([]byte(actualAssetLabel), &actualAssetMap))

	h.AssertEq(t, actualAssetMap, expectedAssetMap)

	for _, asset := range actualAssetMap {
		actualLayer, err := actual.GetLayer(asset.LayerDiffID)
		h.AssertNil(t, err)

		actualContents, err := ioutil.ReadAll(actualLayer)
		h.AssertNil(t, err)

		expectedLayer, err := expected.GetLayer(asset.LayerDiffID)
		h.AssertNil(t, err)

		expectedContents, err := ioutil.ReadAll(expectedLayer)
		h.AssertNil(t, err)

		h.AssertEq(t, actualContents, expectedContents)
	}
}
