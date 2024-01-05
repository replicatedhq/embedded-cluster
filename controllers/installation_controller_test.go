package controllers

import (
	"testing"

	"github.com/k0sproject/dig"
	"gotest.tools/v3/assert"
	"sigs.k8s.io/yaml"
)

func TestMergeValues(t *testing.T) {
	oldData := `
  password: "foo"
  someField: "asdf"
  other: "text"
  overridden: "abcxyz"
  nested:
    nested:
       protect: "testval"
  `
	newData := `
  someField: "newstring"
  other: "text"
  overridden: "this is new"
  nested:
    nested:
      newkey: "newval"
      protect: "newval"
  `
	protect := []string{"password", "overridden", "nested.nested.protect"}

	targetData := `
  password: "foo"
  someField: "newstring"
  nested:
    nested:
      newkey: "newval"
      protect: "testval"
  other: "text"
  overridden: "abcxyz"
  `

	mergedValues, err := MergeValues(oldData, newData, protect)
	if err != nil {
		t.Fail()
	}

	targetDataMap := dig.Mapping{}
	if err := yaml.Unmarshal([]byte(targetData), &targetDataMap); err != nil {
		t.Fail()
	}

	mergedDataMap := dig.Mapping{}
	if err := yaml.Unmarshal([]byte(mergedValues), &mergedDataMap); err != nil {
		t.Fail()
	}

	assert.DeepEqual(t, targetDataMap, mergedDataMap)

}
