// +build acceptance

package assertions

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/docker/docker/client"
)

type AssertionManager struct {
	testObject *testing.T
	dockerCli  *client.Client
}

func NewAssertionManager(testObject *testing.T, dockerCli *client.Client) AssertionManager {
	return AssertionManager{
		testObject: testObject,
		dockerCli:  dockerCli,
	}
}

func (a AssertionManager) Equal(actual, expected interface{}) {
	a.testObject.Helper()

	if diff := cmp.Diff(actual, expected); diff != "" {
		a.testObject.Fatalf(diff)
	}
}

func (a AssertionManager) Nil(actual interface{}) {
	a.testObject.Helper()

	if !isNil(actual) {
		a.testObject.Fatalf("expected nil: %v", actual)
	}
}

func (a AssertionManager) NilWithMessage(actual interface{}, message string) {
	a.testObject.Helper()

	if !isNil(actual) {
		a.testObject.Fatalf("expected nil: %s: %s", actual, message)
	}
}

func (a AssertionManager) NotNil(actual interface{}) {
	a.testObject.Helper()

	if isNil(actual) {
		a.testObject.Fatal("expect not nil")
	}
}

func (a AssertionManager) Contains(actual, expected string) {
	a.testObject.Helper()

	if !strings.Contains(actual, expected) {
		a.testObject.Fatalf(
			"Expected '%s' to contain '%s'\n\nDiff:%s",
			actual,
			expected,
			cmp.Diff(expected, actual),
		)
	}
}

// ContainsWithMessage will fail if expected is not contained within actual, messageFormat will be printed as the
// failure message, with actual interpolated in the message
func (a AssertionManager) ContainsWithMessage(actual, expected, messageFormat string) {
	a.testObject.Helper()

	if !strings.Contains(actual, expected) {
		a.testObject.Fatalf(messageFormat, actual)
	}
}

func (a AssertionManager) ContainsAll(actual string, expected ...string) {
	a.testObject.Helper()

	for _, e := range expected {
		a.Contains(actual, e)
	}
}

func (a AssertionManager) Matches(actual string, pattern *regexp.Regexp) {
	a.testObject.Helper()

	if !pattern.MatchString(actual) {
		a.testObject.Fatalf("Expected '%s' to match regex '%s'", actual, pattern)
	}
}

func (a AssertionManager) MatchesAll(actual string, patterns ...*regexp.Regexp) {
	a.testObject.Helper()

	for _, pattern := range patterns {
		a.Matches(actual, pattern)
	}
}

func (a AssertionManager) NotContain(actual, expected string) {
	a.testObject.Helper()

	if strings.Contains(actual, expected) {
		a.testObject.Fatalf("Expected '%s' not to be in '%s'", expected, actual)
	}
}

// NotContainWithMessage will fail if expected is contained within actual, messageFormat will be printed as the failure
// message, with actual interpolated in the message
func (a AssertionManager) NotContainWithMessage(actual, expected, messageFormat string) {
	a.testObject.Helper()

	if strings.Contains(actual, expected) {
		a.testObject.Fatalf(messageFormat, actual)
	}
}

func (a AssertionManager) errorMatches(actual error, expected string) {
	a.testObject.Helper()

	a.Error(actual)

	if !strings.Contains(actual.Error(), expected) {
		a.testObject.Fatalf(`Expected error to contain "%s", got "%s"`, expected, actual.Error())
	}
}

func (a AssertionManager) Error(actual error) {
	a.testObject.Helper()

	if actual == nil {
		a.testObject.Fatal("Expected an error but got nil")
	}
}

func (a AssertionManager) PackageCreated(name, output string) {
	a.testObject.Helper()

	a.Contains(output, fmt.Sprintf("Successfully created package '%s'", name))
}

func (a AssertionManager) PackagePublished(name, output string) {
	a.testObject.Helper()

	a.Contains(output, fmt.Sprintf("Successfully published package '%s'", name))
}

func isNil(value interface{}) bool {
	return value == nil || (reflect.TypeOf(value).Kind() == reflect.Ptr && reflect.ValueOf(value).IsNil())
}
