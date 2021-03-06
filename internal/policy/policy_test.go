//Copyright 2022 Google LLC
//
//Licensed under the Apache License, Version 2.0 (the "License");
//you may not use this file except in compliance with the License.
//You may obtain a copy of the License at
//
//    https://www.apache.org/licenses/LICENSE-2.0
//
//Unless required by applicable law or agreed to in writing, software
//distributed under the License is distributed on an "AS IS" BASIS,
//WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//See the License for the specific language governing permissions and
//limitations under the License.

package policy

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/rego"
)

func TestNewPolicyEvaluationResults(t *testing.T) {
	r := NewPolicyEvaluationResult()
	if r.Valid == nil {
		t.Errorf("valid is nil; want map")
	}
	if r.Violated == nil {
		t.Errorf("violated is nil; want map")
	}
	if r.Errored == nil {
		t.Errorf("errored is nil; want map")
	}
}

func TestEvaluationResultsGroups(t *testing.T) {
	groupOne := "test-one"
	groupTwo := "test-two"
	groupThree := "test-three"
	r := NewPolicyEvaluationResult()
	r.AddPolicy(&Policy{Group: groupOne, Valid: true})
	r.AddPolicy(&Policy{Group: groupOne, Valid: false})
	r.AddPolicy(&Policy{Group: groupTwo, Valid: true})
	r.AddPolicy(&Policy{Group: groupTwo, Valid: false})
	r.AddPolicy(&Policy{Group: groupThree, Valid: false})
	groups := r.Groups()
	if len(groups) != 3 {
		t.Fatalf("number of groups = %v; want %v", len(groups), 3)
	}
}

func TestAddPolicy(t *testing.T) {
	groupOneName := "groupOne"
	inputs := []*Policy{
		{Group: groupOneName, Valid: true},
		{Group: groupOneName, Valid: true},
		{Group: groupOneName, Valid: false, Violations: []string{"error"}},
		{Group: groupOneName, ProcessingErrors: []error{errors.New("error")}},
	}
	r := NewPolicyEvaluationResult()
	for i := range inputs {
		r.AddPolicy(inputs[i])
	}
	if len(r.Valid[groupOneName]) != 2 {
		t.Errorf("number of valid policies in group %v = %v; want %v", groupOneName, len(r.Valid[groupOneName]), 2)
	}
	if len(r.Violated[groupOneName]) != 1 {
		t.Errorf("number of violated policies in group %v = %v; want %v", groupOneName, len(r.Violated["groupOneName"]), 1)
	}
	if len(r.Errored) != 1 {
		t.Errorf("number of errored policies = %v; want %v", len(r.Errored), 1)
	}
}

func TestViolatedCount(t *testing.T) {
	inputs := []*Policy{
		{Group: "groupOne", Valid: false, Violations: []string{"error"}},
		{Group: "groupOne", Valid: false, Violations: []string{"error"}},
		{Group: "groupTwo", Valid: false, Violations: []string{"error"}},
		{Group: "groupThree", Valid: false, Violations: []string{"error"}},
	}
	r := NewPolicyEvaluationResult()
	for i := range inputs {
		r.AddPolicy(inputs[i])
	}
	violatedCount := r.ViolatedCount()
	if violatedCount != len(inputs) {
		t.Errorf("violatedCount = %v; want %v", violatedCount, len(inputs))
	}
}

func TestValidCount(t *testing.T) {
	inputs := []*Policy{
		{Group: "groupOne", Valid: true},
		{Group: "groupOne", Valid: true},
		{Group: "groupTwo", Valid: true},
		{Group: "groupTwo", Valid: true},
		{Group: "groupThree", Valid: true},
	}
	r := NewPolicyEvaluationResult()
	for i := range inputs {
		r.AddPolicy(inputs[i])
	}
	validCount := r.ValidCount()
	if validCount != len(inputs) {
		t.Errorf("validCount = %v; want %v", validCount, len(inputs))
	}
}

func TestErroredCount(t *testing.T) {
	inputs := []*Policy{
		{Group: "groupOne", ProcessingErrors: []error{errors.New("error")}},
		{Group: "groupTwo", ProcessingErrors: []error{errors.New("error")}},
		{Group: "groupThree", ProcessingErrors: []error{errors.New("error")}},
	}
	r := NewPolicyEvaluationResult()
	for i := range inputs {
		r.AddPolicy(inputs[i])
	}
	erroredCount := r.ErroredCount()
	if erroredCount != len(inputs) {
		t.Errorf("erroredCount = %v; want %v", erroredCount, len(inputs))
	}
}

func TestCompile(t *testing.T) {
	policyFiles := []*PolicyFile{
		{"test_one.rego", "folder/test_one.rego", `
package test_one
p = 1`},
		{"test_two.rego", "folder/test_two.rego", `
package bla.test_two
p = 2`}}
	pa := NewPolicyAgent(context.Background())

	err := pa.Compile(policyFiles)
	if err != nil {
		t.Fatalf("err = %q; want nil", err)
	}
	if pa.compiler == nil {
		t.Fatalf("compiler = nil; want compiler")
	}
	if len(pa.compiler.Modules) != len(policyFiles) {
		t.Errorf("number of compiled policies = %d; want %d", len(pa.compiler.Modules), len(policyFiles))
	}
	for _, file := range policyFiles {
		if _, ok := pa.compiler.Modules[file.FullName]; !ok {
			t.Errorf("compiler has no module for file %s", file)
		}
	}
}

func TestCompile_parseError(t *testing.T) {
	policyFiles := []*PolicyFile{
		{"test_one.rego", "folder/test_one.rego", `
bla bla`}}
	pa := PolicyAgent{}
	err := pa.Compile(policyFiles)
	if err == nil {
		t.Errorf("err is nil; want error")
	}
}

func TestParseCompiled(t *testing.T) {
	goodPackage := "gke.policy.testOk"
	policyContentOk := fmt.Sprintf("# METADATA\n"+
		"# title: TestTitle\n"+
		"# description: TestDescription\n"+
		"# custom:\n"+
		"#   group: TestGroup\n"+
		"package %s\n"+
		"p = 1", goodPackage)
	policyContentBadMeta := `# METADATA
# title:  TestTitle
package gke.policy.badMeta
p = 1`
	policyContentBadMetaTwo := `# METADATA
# title: TestTitle
# description: TestDescription
package gke.policy.badMeta
p = 1`

	policyFiles := []*PolicyFile{
		{"test_one.rego", "folder/test_one.rego", policyContentOk},
		{"test_two.rego", "folder/test_two.rego", policyContentBadMeta},
		{"test_three.rego", "folder/test_three.rego", policyContentBadMetaTwo},
	}
	pa := PolicyAgent{}
	if err := pa.Compile(policyFiles); err != nil {
		t.Fatalf("err is %s; expected nil", err)
	}
	policies, errors := pa.ParseCompiled()
	if len(policies) != 1 {
		t.Fatalf("len(policies) = %v; want %v", len(policies), 1)
	}
	if len(errors) != 2 {
		t.Fatalf("len(errors) = %v; want %v", len(policies), 2)
	}
	if policies[0].Name != goodPackage {
		t.Errorf("policy[0] name = %v; want %v", policies[0].Name, goodPackage)
	}
}

func TestParseCompiled_noCompiler(t *testing.T) {
	pa := PolicyAgent{}
	if _, err := pa.ParseCompiled(); err == nil {
		t.Fatalf("err is nil; want error")
	}
}

func TestWithFiles(t *testing.T) {
	packageOne := regoPolicyPackage + ".package_one"
	titleOne := "TitleOne"
	contentOne := fmt.Sprintf("# METADATA\n"+
		"# title: %s\n"+
		"# description: Test\n"+
		"# custom:\n"+
		"#   group: Test\n"+
		"package %s\n"+
		"p = 1", titleOne, packageOne)
	packageTwo := regoPolicyPackage + ".package_three"
	titleTwo := "TitleTwo"
	contentTwo := fmt.Sprintf("# METADATA\n"+
		"# title: %s\n"+
		"# description: Test\n"+
		"# custom:\n"+
		"#   group: Test\n"+
		"package %s\n"+
		"p = 1", titleTwo, packageTwo)
	contentThree := fmt.Sprintf("# METADATA\n" +
		"# title: TitleThree\n" +
		"# description: Test\n" +
		"# custom:\n" +
		"#   group: Test\n" +
		"package gke.something.invalid\n" +
		"p = 1")
	policyFiles := []*PolicyFile{
		{"test_one.rego", "folder/test_one.rego", contentOne},
		{"test_two.rego", "folder/test_two.rego", contentTwo},
		{"test_three.rego", "folder/test_three.rego", contentThree},
		{"test_one_test.rego", "folder/test_one_test.rego", contentThree},
	}
	pa := PolicyAgent{}
	if err := pa.WithFiles(policyFiles); err != nil {
		t.Fatalf("error = %v; want nil", err)
	}
	if len(pa.compiled) != 2 {
		t.Fatalf("len(pa.compiled) = %v; want %v", len(pa.compiled), 2)
	}
	if pa.compiled[packageOne].Title != titleOne {
		t.Errorf("Policy %q title = %v; want %v", packageOne, pa.compiled[packageOne].Title, titleOne)
	}
	if pa.compiled[packageTwo].Title != titleTwo {
		t.Errorf("Policy %q title = %v; want %v", packageTwo, pa.compiled[packageTwo].Title, titleTwo)
	}
}

func TestProcessRegoResultSet(t *testing.T) {
	policyOneCompiled := &Policy{
		Name:        regoPolicyPackage + ".policy_one",
		File:        "rego/policy_one.rego",
		Title:       "Policy One test",
		Description: "This is just for test",
		Group:       "policy_one",
	}
	policyOneResult := rego.Result{
		Expressions: []*rego.ExpressionValue{
			{Value: map[string]interface{}{
				"valid":     true,
				"violation": []interface{}{},
			}},
		},
		Bindings: map[string]interface{}{
			"name": "policy_one",
		},
	}
	policyTwoCompiled := &Policy{
		Name:        regoPolicyPackage + ".policy_two",
		File:        "rego/policy_two.rego",
		Title:       "Policy Two test",
		Description: "This is just for test",
		Group:       "policy_two",
	}
	policyTwoResult := rego.Result{
		Expressions: []*rego.ExpressionValue{
			{Value: map[string]interface{}{
				"valid":     false,
				"violation": []interface{}{"error"},
			}},
		},
		Bindings: map[string]interface{}{
			"name": "policy_two",
		},
	}
	policyThreeCompiled := &Policy{
		Name:        regoPolicyPackage + ".policy_three",
		File:        "rego/policy_three.rego",
		Title:       "Policy Three test",
		Description: "This is just for test",
	}
	policyThreeResult := rego.Result{
		Expressions: []*rego.ExpressionValue{
			{Value: map[string]interface{}{
				"valid": false,
			}},
		},
		Bindings: map[string]interface{}{
			"name": "policy_three",
		},
	}
	resultSet := []rego.Result{policyOneResult, policyTwoResult, policyThreeResult}
	pa := PolicyAgent{}
	pa.compiled = map[string]*Policy{
		policyOneCompiled.Name:   policyOneCompiled,
		policyTwoCompiled.Name:   policyTwoCompiled,
		policyThreeCompiled.Name: policyThreeCompiled,
	}

	result, err := pa.processRegoResultSet(resultSet)
	if err != nil {
		t.Fatalf("got error; expected nil")
	}
	if _, ok := result.Valid["policy_one"]; !ok {
		t.Errorf("valid policy not grouped under %v key", "policy_one")
	}
	if _, ok := result.Violated["policy_two"]; !ok {
		t.Errorf("violated policy not grouped under %v key", "policy_two")
	}
	if len(result.Errored) != 1 {
		t.Fatalf("number of errored policies = %v; want %v", len(result.Errored), 1)
	}
}

func TestGetResultDataForEval(t *testing.T) {
	input := []rego.Result{
		{Expressions: []*rego.ExpressionValue{{Value: "test"}},
			Bindings: map[string]interface{}{"name": "test"}},
		{Expressions: []*rego.ExpressionValue{{Text: "test"}}},
		{Bindings: map[string]interface{}{"name": "test"}},
		{},
	}
	expected := []interface{}{
		rego.Result{
			Expressions: []*rego.ExpressionValue{{Value: "test"}},
			Bindings:    map[string]interface{}{"name": "test"}},
		nil,
		nil,
		nil,
	}
	for i := range input {
		value, bindings, err := getResultDataForEval(input[i])
		if err == nil {
			expectedResult := expected[i].(rego.Result)
			if !reflect.DeepEqual(value, expectedResult.Expressions[0].Value) {
				t.Errorf("value = %v; want %v", value, expectedResult.Expressions[0].Value)
			}
			if !reflect.DeepEqual(bindings["name"], expectedResult.Bindings["name"]) {
				t.Errorf("bindings[name] = %v; want %v", bindings["name"], expectedResult.Bindings["name"])
			}
		} else {
			if expected[i] != nil {
				t.Errorf("did not expect error; got error")
			}
		}
	}
}

func TestMapExpressionBindings(t *testing.T) {
	bindings := []map[string]interface{}{
		{"name": "policy_name"},
		{"name": 20},
		{"bogus": "value"},
	}
	expected := []interface{}{
		"policy_name",
		nil,
		nil,
	}
	result := RegoEvaluationResult{}
	for i := range bindings {
		err := result.mapExpressionBindings(bindings[i])
		if err == nil {
			if result.Name != expected[i] {
				t.Errorf("name = %v; want %v", result.Name, expected[i])
			}
		} else {
			if expected[i] != nil {
				t.Errorf("did not expect error; got error")
			}
		}
	}
}

func TestMapExpressionValue(t *testing.T) {
	input := map[string]interface{}{
		"valid":     true,
		"violation": []interface{}{"violation"},
	}
	expectedValid := true
	expectedViolations := []string{"violation"}

	result := RegoEvaluationResult{}
	if err := result.mapExpressionValue(input); err != nil {
		t.Errorf("err = %q; want nil", err)
	}
	if result.Valid != expectedValid {
		t.Errorf("valid = %v; want %v", result.Valid, expectedValid)
	}
	if !reflect.DeepEqual(result.Violations, expectedViolations) {
		t.Errorf("valid = %v; want %v", result.Violations, expectedViolations)
	}
}

func TestParseRegoPolicyData(t *testing.T) {
	input := map[string]interface{}{
		"valid":     true,
		"violation": []interface{}{"violation"},
	}
	expectedValid := true
	expectedViolations := []string{"violation"}

	valid, violations, err := parseRegoPolicyData(input)
	if err != nil {
		t.Errorf("err = %q; want nil", err)
	}
	if valid != input["valid"] {
		t.Errorf("valid = %v; want %v", valid, expectedValid)
	}
	if !reflect.DeepEqual(violations, expectedViolations) {
		t.Errorf("violations = %v; want %v", violations, expectedViolations)
	}
}

func TestMapModule(t *testing.T) {
	file := "folder/test_one.rego"
	pkg := "gke.policy.test"
	title := "This is title"
	desc := "This is long description"
	group := "TestGroup"

	content := fmt.Sprintf("# METADATA\n"+
		"# title: %s\n"+
		"# description: %s\n"+
		"# custom:\n"+
		"#   group: %s\n"+
		"package %s\n"+
		"p = 1", title, desc, group, pkg)

	modules := map[string]string{file: content}
	compiler := ast.MustCompileModulesWithOpts(modules,
		ast.CompileOpts{ParserOptions: ast.ParserOptions{ProcessAnnotation: true}})
	module := compiler.Modules[file]
	policy := Policy{}
	policy.MapModule(module)

	if policy.Name != pkg {
		t.Errorf("name = %v; want %v", policy.Name, pkg)
	}
	if policy.File != file {
		t.Errorf("file = %v; want %v", policy.File, file)
	}
	if policy.Title != title {
		t.Errorf("title = %v; want %v", policy.Title, title)
	}
	if policy.Description != desc {
		t.Errorf("description = %v; want %v", policy.Description, desc)
	}
	if policy.Group != group {
		t.Errorf("group = %v; want %v", policy.Group, group)
	}
}

func TestMetadataErrors(t *testing.T) {
	input := []Policy{
		{Title: "title", Description: "description", Group: "group"},
		{Title: "title", Description: "description"},
		{Title: "title"},
		{},
	}
	expErrCnt := []int{
		0,
		1,
		2,
		3,
	}
	for i := range input {
		errors := input[i].MetadataErrors()
		if len(errors) != expErrCnt[i] {
			t.Errorf("error cnt = %v; want %v", len(errors), expErrCnt[i])
		}
	}
}

func TestGetBoolFromInterfaceMap(t *testing.T) {
	inputName := "test"
	inputMap := map[string]interface{}{"test": true}
	expected := true

	result, err := getBoolFromInterfaceMap(inputName, inputMap)
	if err != nil {
		t.Errorf("err = %q; want nil", err)
	}
	if result != expected {
		t.Errorf("result = %v; want %v", result, expected)
	}
}

func TestGetBoolFromInterfaceMap_negative(t *testing.T) {
	inputNames := []string{"testTwo", "missing"}
	inputMaps := []map[string]interface{}{{"testTwo": 101}, nil}
	for i := range inputNames {
		_, err := getBoolFromInterfaceMap(inputNames[i], inputMaps[i])
		if err == nil {
			t.Errorf("err = nil; want error")
		}
	}
}

func TestGetStringListFromInterfaceMap(t *testing.T) {
	inputName := "test"
	inputMap := map[string]interface{}{"test": []interface{}{"str1", "str2"}}
	expected := []string{"str1", "str2"}

	result, err := getStringListFromInterfaceMap(inputName, inputMap)
	if err != nil {
		t.Errorf("err = %q; want nil", err)
	}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("result = %v; want %v", result, expected)
	}
}

func TestGetStringListFromInterfaceMap_negative(t *testing.T) {
	inputNames := []string{"testTwo", "testThree", "missing"}
	inputMaps := []map[string]interface{}{
		{"testTwo": nil},
		{"testThree": []interface{}{"str1", 100}},
		nil}
	for i := range inputNames {
		_, err := getStringListFromInterfaceMap(inputNames[i], inputMaps[i])
		if err == nil {
			t.Errorf("err = nil; want error")
		}
	}
}
