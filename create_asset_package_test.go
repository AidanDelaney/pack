package pack_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"testing"

	testmocks2 "github.com/buildpacks/pack/internal/asset/testmocks"

	"github.com/buildpacks/imgutil/fakes"
	"github.com/google/go-containerregistry/pkg/name"

	"github.com/golang/mock/gomock"
	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"

	fakes2 "github.com/buildpacks/pack/internal/asset/fakes"
	"github.com/buildpacks/pack/internal/blob"
	"github.com/buildpacks/pack/internal/dist"
	"github.com/buildpacks/pack/internal/oci"

	"github.com/buildpacks/pack"
	ilogging "github.com/buildpacks/pack/internal/logging"
	"github.com/buildpacks/pack/logging"
	h "github.com/buildpacks/pack/testhelpers"
	"github.com/buildpacks/pack/testmocks"
)

func TestCreateAssetCacheCommand(t *testing.T) {
	spec.Run(t, "CreateAssetCacheCommand", testCreateAssetCacheCommand, spec.Parallel(), spec.Report(report.Terminal{}))
}

func testCreateAssetCacheCommand(t *testing.T, when spec.G, it spec.S) {
	var (
		client           *pack.Client
		assert           = h.NewAssertionManager(t)
		logger           logging.Logger
		mockController   *gomock.Controller
		mockDownloader   *testmocks.MockDownloader
		mockImageFactory *testmocks.MockImageFactory
		mockImageFetcher *testmocks.MockImageFetcher
		mockDockerClient *testmocks.MockCommonAPIClient
		//fakeImage        *fakes.Image
		out    bytes.Buffer
		tmpDir string

		firstAsset = dist.AssetInfo{ID: "first-asset",
			Name:    "First AssetInfo",
			Sha256:  "first-sha256",
			Stacks:  []string{"io.buildpacks.stacks.bionic"},
			URI:     "https://first-asset-uri",
			Version: "1.2.3",
		}
		firstAssetReplace = dist.AssetInfo{
			ID:      "first-asset-replace",
			Name:    "First AssetInfo Replace",
			Sha256:  "first-sha256",
			Stacks:  []string{"io.buildpacks.stacks.bionic"},
			URI:     "https://first-asset-replace-uri",
			Version: "1.2.3",
		}
		secondAsset = dist.AssetInfo{
			ID:      "second-asset",
			Name:    "Second AssetInfo",
			Sha256:  "second-sha256",
			Stacks:  []string{"io.buildpacks.stacks.bionic"},
			URI:     "https://second-asset-uri",
			Version: "4.5.6",
		}
		thirdAsset = dist.AssetInfo{
			ID:      "third-asset",
			Name:    "Third AssetInfo",
			Sha256:  "third-sha256",
			Stacks:  []string{"io.buildpacks.stacks.bionic"},
			Version: "7.8.9",
		}
		firstAssetBlob        dist.Blob
		firstAssetReplaceBlob dist.Blob
		secondAssetBlob       dist.Blob
	)
	it.Before(func() {
		var err error
		logger = ilogging.NewLogWithWriters(&out, &out, ilogging.WithVerbose())
		mockController = gomock.NewController(t)
		mockDownloader = testmocks.NewMockDownloader(mockController)
		mockImageFetcher = testmocks.NewMockImageFetcher(mockController)
		mockImageFactory = testmocks.NewMockImageFactory(mockController)
		mockDockerClient = testmocks.NewMockCommonAPIClient(mockController)
		client, err = pack.NewClient(
			pack.WithLogger(logger),
			pack.WithDownloader(mockDownloader),
			pack.WithImageFactory(mockImageFactory),
			pack.WithFetcher(mockImageFetcher),
			pack.WithDockerClient(mockDockerClient),
		)
		assert.Nil(err)

		tmpDir, err = ioutil.TempDir("", "create-asset-package-command-test")
		assert.Nil(err)

		firstAssetBlob = fakes2.NewFakeBlob("first asset contents")
		firstAssetReplaceBlob = fakes2.NewFakeBlob("first replace asset contents")
		secondAssetBlob = fakes2.NewFakeBlob("second asset contents")
	})
	when("#CreateAssetPackage", func() {
		when("output format is file", func() {
			it("writes asset package as a file", func() {
				ctx := context.TODO()

				mockDownloader.EXPECT().Download(gomock.Any(), firstAsset.URI, gomock.Any(), gomock.Any()).Return(firstAssetBlob, nil)
				mockDownloader.EXPECT().Download(gomock.Any(), secondAsset.URI, gomock.Any(), gomock.Any()).Return(secondAssetBlob, nil)

				imagePath := filepath.Join(tmpDir, "test-cache")
				assert.Succeeds(client.CreateAssetPackage(ctx, pack.CreateAssetPackageOptions{
					ImageName: imagePath,
					Assets:    []dist.AssetInfo{firstAsset, secondAsset, thirdAsset},
					Publish:   false,
					OS:        "linux",
					Format:    "file",
				}))

				// verify contents of asset image
				testCacheBlob := blob.NewBlob(filepath.Join(imagePath))
				pkg, err := oci.NewLayoutPackage(testCacheBlob)
				assert.Nil(err)

				mdJSON, err := pkg.Label("io.buildpacks.asset.layers")
				assert.Nil(err)

				var md dist.AssetMap
				assert.Succeeds(json.Unmarshal([]byte(mdJSON), &md))
				assert.Equal(md, dist.AssetMap{
					"first-sha256": dist.AssetValue{
						ID:          "first-asset",
						Name:        "First AssetInfo",
						LayerDiffID: "sha256:ac4ae299af0610acf496c05bc740de64222eb110d6aaf0c12916ebdefb83a54f",
						Stacks:      []string{"io.buildpacks.stacks.bionic"},
						URI:         "https://first-asset-uri",
						Version:     "1.2.3",
					}, "second-sha256": dist.AssetValue{
						ID:          "second-asset",
						Name:        "Second AssetInfo",
						LayerDiffID: "sha256:85bbbc8202dcbe8b5b7d6a5cffbd0da8faa59d5c406c6bcc3a1156f4e58b2c6a",
						Stacks:      []string{"io.buildpacks.stacks.bionic"},
						URI:         "https://second-asset-uri",
						Version:     "4.5.6",
					},
				})

				firstLayer, err := pkg.GetLayer(md[firstAsset.Sha256].LayerDiffID)
				assert.Nil(err)
				h.AssertOnTarReaderEntry(t, firstLayer, fmt.Sprintf("/cnb/assets/%s", firstAsset.Sha256), h.ContentContains("first asset contents"))

				secondLayer, err := pkg.GetLayer(md[secondAsset.Sha256].LayerDiffID)
				assert.Nil(err)
				h.AssertOnTarReaderEntry(t, secondLayer, fmt.Sprintf("/cnb/assets/%s", secondAsset.Sha256), h.ContentContains("second asset contents"))
			})
		})
		when("output format is an image", func() {
			it("writes asset package as an image", func() {
				ctx := context.TODO()
				imageName := "test-cache-image"
				imgRef, err := name.NewTag(imageName)
				assert.Nil(err)

				fakeImage := fakes.NewImage(imgRef.Name(), "", nil)
				mockImageFactory.EXPECT().NewImage(imgRef.Name(), true, "linux").Return(fakeImage, nil)
				mockDownloader.EXPECT().Download(gomock.Any(), firstAsset.URI, gomock.Any(), gomock.Any()).Return(firstAssetBlob, nil)
				mockDownloader.EXPECT().Download(gomock.Any(), secondAsset.URI, gomock.Any(), gomock.Any()).Return(secondAssetBlob, nil)

				assert.Succeeds(client.CreateAssetPackage(ctx, pack.CreateAssetPackageOptions{
					ImageName: imgRef.Name(),
					Assets:    []dist.AssetInfo{firstAsset, secondAsset, thirdAsset},
					Publish:   false,
					OS:        "linux",
					Format:    "image",
				}))

				// verify contents of asset image

				mdJSON, err := fakeImage.Label("io.buildpacks.asset.layers")
				assert.Nil(err)
				var md dist.AssetMap
				assert.Succeeds(json.Unmarshal([]byte(mdJSON), &md))
				assert.Equal(md, dist.AssetMap{
					"first-sha256": dist.AssetValue{
						ID:          "first-asset",
						Name:        "First AssetInfo",
						LayerDiffID: "sha256:ac4ae299af0610acf496c05bc740de64222eb110d6aaf0c12916ebdefb83a54f",
						Stacks:      []string{"io.buildpacks.stacks.bionic"},
						URI:         "https://first-asset-uri",
						Version:     "1.2.3",
					}, "second-sha256": dist.AssetValue{
						ID:          "second-asset",
						Name:        "Second AssetInfo",
						LayerDiffID: "sha256:85bbbc8202dcbe8b5b7d6a5cffbd0da8faa59d5c406c6bcc3a1156f4e58b2c6a",
						Stacks:      []string{"io.buildpacks.stacks.bionic"},
						URI:         "https://second-asset-uri",
						Version:     "4.5.6",
					},
				})

				assert.Equal(fakeImage.IsSaved(), true)

				os, err := fakeImage.OS()
				assert.Nil(err)
				assert.Equal(os, "linux")

				firstLayer, err := fakeImage.GetLayer(md[firstAsset.Sha256].LayerDiffID)
				assert.Nil(err)
				h.AssertOnTarReaderEntry(t, firstLayer, fmt.Sprintf("/cnb/assets/%s", firstAsset.Sha256), h.ContentContains("first asset contents"))

				secondLayer, err := fakeImage.GetLayer(md[secondAsset.Sha256].LayerDiffID)
				assert.Nil(err)
				h.AssertOnTarReaderEntry(t, secondLayer, fmt.Sprintf("/cnb/assets/%s", secondAsset.Sha256), h.ContentContains("second asset contents"))
			})

			when("os is windows", func() {
				it("writes a windows image", func() {
					ctx := context.TODO()
					imageName := "test-cache-image-windows"
					imgRef, err := name.NewTag(imageName)
					assert.Nil(err)

					fakeImage := fakes.NewImage(imgRef.Name(), "", nil)
					mockImageFactory.EXPECT().NewImage(imgRef.Name(), true, "windows").Return(fakeImage, nil)
					mockDownloader.EXPECT().Download(gomock.Any(), firstAsset.URI, gomock.Any(), gomock.Any()).Return(firstAssetBlob, nil)

					assert.Succeeds(client.CreateAssetPackage(ctx, pack.CreateAssetPackageOptions{
						ImageName: imgRef.Name(),
						Assets:    []dist.AssetInfo{firstAsset},
						Publish:   false,
						OS:        "windows",
						Format:    "image",
					}))
					mdJSON, err := fakeImage.Label("io.buildpacks.asset.layers")
					assert.Nil(err)

					var md dist.AssetMap
					assert.Succeeds(json.Unmarshal([]byte(mdJSON), &md))
					assert.Equal(md, dist.AssetMap{
						"first-sha256": dist.AssetValue{
							ID:          "first-asset",
							Name:        "First AssetInfo",
							LayerDiffID: "sha256:c552b5f9e912a7dc2a0cff4fe41001a867dd6a7d52e363247445ddf0c46784c7",
							Stacks:      []string{"io.buildpacks.stacks.bionic"},
							URI:         "https://first-asset-uri",
							Version:     "1.2.3",
						},
					})

					assert.Equal(fakeImage.IsSaved(), true)

					// windows layer will be added by image factory
					assert.Equal(fakeImage.NumberOfAddedLayers(), 1)

					firstLayer, err := fakeImage.GetLayer(md[firstAsset.Sha256].LayerDiffID)
					assert.Nil(err)
					h.AssertOnTarReaderEntry(t, firstLayer, fmt.Sprintf("Files/cnb/assets/%s", firstAsset.Sha256), h.ContentContains("first asset contents"))
				})
			})
		})
		when("publish is true", func() {
			it("creates a remote image", func() {
				ctx := context.TODO()
				imageName := "test-cache-image-windows"
				imgRef, err := name.NewTag(imageName)
				assert.Nil(err)

				fakeImage := fakes.NewImage(imgRef.Name(), "", nil)
				mockImageFactory.EXPECT().NewImage(imgRef.Name(), false, "windows").Return(fakeImage, nil)
				mockDownloader.EXPECT().Download(gomock.Any(), firstAsset.URI, gomock.Any(), gomock.Any()).Return(firstAssetBlob, nil)

				assert.Succeeds(client.CreateAssetPackage(ctx, pack.CreateAssetPackageOptions{
					ImageName: imgRef.Name(),
					Assets:    []dist.AssetInfo{firstAsset},
					Publish:   true,
					OS:        "windows",
					Format:    "image",
				}))
				assert.Equal(fakeImage.IsSaved(), true)
			})
		})
		when("two assets have the same sh256 value", func() {
			it("last asset wins", func() {
				ctx := context.TODO()
				imageName := "test-cache-image"
				imgRef, err := name.NewTag(imageName)
				assert.Nil(err)

				fakeImage := fakes.NewImage(imgRef.Name(), "", nil)
				mockImageFactory.EXPECT().NewImage(imgRef.Name(), true, "linux").Return(fakeImage, nil)
				mockDownloader.EXPECT().Download(gomock.Any(), firstAssetReplace.URI, gomock.Any(), gomock.Any()).Return(firstAssetReplaceBlob, nil)

				assert.Succeeds(client.CreateAssetPackage(ctx, pack.CreateAssetPackageOptions{
					ImageName: imgRef.Name(),
					Assets:    []dist.AssetInfo{firstAsset, firstAssetReplace},
					Publish:   false,
					OS:        "linux",
					Format:    "image",
				}))

				mdJSON, err := fakeImage.Label("io.buildpacks.asset.layers")
				assert.Nil(err)
				var md dist.AssetMap
				assert.Succeeds(json.Unmarshal([]byte(mdJSON), &md))
				assert.Equal(md, dist.AssetMap{
					"first-sha256": dist.AssetValue{
						ID:          "first-asset-replace",
						Name:        "First AssetInfo Replace",
						Stacks:      []string{"io.buildpacks.stacks.bionic"},
						LayerDiffID: "sha256:3b3d445d01df824e1ce27a6573270eeaa0c85f1b3335ec1aad85d6126193fb41",
						URI:         "https://first-asset-replace-uri",
						Version:     "1.2.3",
					},
				})
				assert.Equal(fakeImage.IsSaved(), true)

				os, err := fakeImage.OS()
				assert.Nil(err)
				assert.Equal(os, "linux")

				firstLayer, err := fakeImage.GetLayer(md[firstAssetReplace.Sha256].LayerDiffID)
				assert.Nil(err)
				h.AssertOnTarReaderEntry(t, firstLayer, fmt.Sprintf("/cnb/assets/%s", firstAssetReplace.Sha256), h.ContentContains("first replace asset contents"))
			})
		})
	})

	when("error cases", func() {
		when("passed an unknown OS", func() {
			it("errors with a helpful message", func() {
				ctx := context.TODO()
				err := client.CreateAssetPackage(ctx, pack.CreateAssetPackageOptions{
					ImageName: "fail-image-ref",
					Assets:    []dist.AssetInfo{},
					Publish:   false,
					OS:        "unknown-os",
					Format:    "image",
				})
				assert.ErrorContains(err, "unable to create layer tar writer")
			})
		})
		when("unable to create base image", func() {
			it("errors with a helpful message", func() {
				ctx := context.TODO()
				imgName := "fail-image-ref"
				mockImageFactory.EXPECT().NewImage(imgName, true, "linux").Return(nil, errors.New("image create error"))
				err := client.CreateAssetPackage(ctx, pack.CreateAssetPackageOptions{
					ImageName: imgName,
					Assets:    []dist.AssetInfo{},
					Publish:   false,
					OS:        "linux",
					Format:    "image",
				})

				assert.ErrorContains(err, "unable to create asset package base image")
			})
		})

		when("unable to download asset", func() {
			it.Before(func() {
				var err error
				client, err = pack.NewClient(
					pack.WithLogger(logger),
					pack.WithDownloader(testmocks.NewFakeDownloader(errors.New("download error"))),
					pack.WithImageFactory(mockImageFactory),
					pack.WithFetcher(mockImageFetcher),
					pack.WithDockerClient(mockDockerClient),
				)
				assert.Nil(err)
			})
			it("errors with a helpful message", func() {
				ctx := context.TODO()
				imgName := "fail-image-ref"

				fakeImage := fakes.NewImage(imgName, "", nil)
				mockImageFactory.EXPECT().NewImage(imgName, true, "linux").Return(fakeImage, nil)
				err := client.CreateAssetPackage(ctx, pack.CreateAssetPackageOptions{
					ImageName: imgName,
					Assets:    []dist.AssetInfo{firstAsset},
					Publish:   false,
					OS:        "linux",
					Format:    "image",
				})

				assert.ErrorContains(err, "unable to download assets")
			})
		})
		when("unable to add assets to final image", func() {
			var mockBlob *testmocks2.MockBlob
			it.Before(func() {
				mockBlob = testmocks2.NewMockBlob(mockController)
			})
			it("errors with a helpful message", func() {
				ctx := context.TODO()

				mockDownloader.EXPECT().Download(gomock.Any(), firstAsset.URI, gomock.Any(), gomock.Any()).Return(mockBlob, nil)
				mockBlob.EXPECT().Open().Return(nil, errors.New("open blob error"))
				imagePath := filepath.Join(tmpDir, "test-cache")
				err := client.CreateAssetPackage(ctx, pack.CreateAssetPackageOptions{
					ImageName: imagePath,
					Assets:    []dist.AssetInfo{firstAsset},
					Publish:   false,
					OS:        "linux",
					Format:    "file",
				})

				assert.ErrorContains(err, "unable to add asset blobs to assets package")
			})
		})
	})
}
