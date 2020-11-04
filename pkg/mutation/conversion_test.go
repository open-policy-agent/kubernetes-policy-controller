package mutation_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	mutationsv1alpha1 "github.com/open-policy-agent/gatekeeper/apis/mutations/v1alpha1"
	"github.com/open-policy-agent/gatekeeper/pkg/mutation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestAssignToMutator(t *testing.T) {
	assign := &mutationsv1alpha1.Assign{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "namespace",
		},
		Spec: mutationsv1alpha1.AssignSpec{
			ApplyTo: []mutationsv1alpha1.ApplyTo{
				{
					Groups:   []string{"group1", "group2"},
					Kinds:    []string{"kind1", "kind2", "kind3"},
					Versions: []string{"version1"},
				},
				{
					Groups:   []string{"group3", "group4"},
					Kinds:    []string{"kind4", "kind2", "kind3"},
					Versions: []string{"version1"},
				},
			},
			Match:      mutationsv1alpha1.Match{},
			Location:   "spec.foo",
			Parameters: mutationsv1alpha1.Parameters{},
		},
	}

	mutatorWithSchema, err := mutation.MutatorForAssign(assign)
	if err != nil {
		t.Fatalf("MutatorForAssign failed, %v", err)
	}

	bindings := mutatorWithSchema.SchemaBindings()
	expectedBindings := []mutation.SchemaBinding{
		{
			Groups:   []string{"group1", "group2"},
			Kinds:    []string{"kind1", "kind2", "kind3"},
			Versions: []string{"version1"},
		},
		{
			Groups:   []string{"group3", "group4"},
			Kinds:    []string{"kind4", "kind2", "kind3"},
			Versions: []string{"version1"},
		},
	}

	if !cmp.Equal(bindings, expectedBindings) {
		t.Errorf("Bindings are not as expected: %s", cmp.Diff(bindings, expectedBindings))
	}

}

func TestAssignMetadataToMutator(t *testing.T) {
	assignMeta := &mutationsv1alpha1.AssignMetadata{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "namespace",
		},
		Spec: mutationsv1alpha1.AssignMetadataSpec{
			Match:      mutationsv1alpha1.Match{},
			Location:   "spec.foo",
			Parameters: mutationsv1alpha1.MetadataParameters{},
		},
	}

	_, err := mutation.MutatorForAssignMetadata(assignMeta)
	if err != nil {
		t.Fatalf("MutatorForAssignMetadata for failed, %v", err)
	}
}

func TestAssignHasDiff(t *testing.T) {
	first := &mutationsv1alpha1.Assign{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "namespace",
		},
		Spec: mutationsv1alpha1.AssignSpec{
			ApplyTo: []mutationsv1alpha1.ApplyTo{
				{
					Groups:   []string{"group1", "group2"},
					Kinds:    []string{"kind1", "kind2", "kind3"},
					Versions: []string{"version1"},
				},
				{
					Groups:   []string{"group3", "group4"},
					Kinds:    []string{"kind4", "kind2", "kind3"},
					Versions: []string{"version1"},
				},
			},
			Match:      mutationsv1alpha1.Match{},
			Location:   "spec.foo",
			Parameters: mutationsv1alpha1.Parameters{},
		},
	}
	// This is normally filled during the serialization
	gvk := schema.GroupVersionKind{
		Kind:    "kindname",
		Group:   "groupname",
		Version: "versionname",
	}
	first.APIVersion, first.Kind = gvk.ToAPIVersionAndKind()

	second := first.DeepCopy()

	table := []struct {
		tname        string
		modify       func(*mutationsv1alpha1.Assign)
		areDifferent bool
	}{
		{
			"same",
			func(a *mutationsv1alpha1.Assign) {},
			false,
		},
		{
			"differentApplyTo",
			func(a *mutationsv1alpha1.Assign) {
				a.Spec.ApplyTo[1].Kinds[0] = "kind"
			},
			true,
		},
		{
			"differentLocation",
			func(a *mutationsv1alpha1.Assign) {
				a.Spec.Location = "bar"
			},
			true,
		},
		{
			"differentParameters",
			func(a *mutationsv1alpha1.Assign) {
				a.Spec.Parameters.IfIn = []string{"Foo", "Bar"}
			},
			true,
		},
	}

	for _, tc := range table {
		t.Run(tc.tname, func(t *testing.T) {
			secondAssign := second.DeepCopy()
			tc.modify(secondAssign)
			firstMutator, err := mutation.MutatorForAssign(first)
			if err != nil {
				t.Fatal(tc.tname, "Failed to convert first to mutator", err)
			}
			secondMutator, err := mutation.MutatorForAssign(secondAssign)
			if err != nil {
				t.Fatal(tc.tname, "Failed to convert second to mutator", err)
			}
			hasDiff := firstMutator.HasDiff(secondMutator)
			hasDiff1 := secondMutator.HasDiff(firstMutator)
			if hasDiff != hasDiff1 {
				t.Error(tc.tname, "Diff first from second is different from second to first")
			}
			if hasDiff != tc.areDifferent {
				t.Errorf("test %s: Expected to be different: %v, diff result is %v", tc.tname, tc.areDifferent, hasDiff)
			}
		})
	}
}

func TestAssignMetadataHasDiff(t *testing.T) {
	first := &mutationsv1alpha1.AssignMetadata{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "namespace",
		},
		Spec: mutationsv1alpha1.AssignMetadataSpec{
			Match:      mutationsv1alpha1.Match{},
			Location:   "spec.foo",
			Parameters: mutationsv1alpha1.MetadataParameters{},
		},
	}

	// This is normally filled during the serialization
	gvk := schema.GroupVersionKind{
		Kind:    "kindname",
		Group:   "groupname",
		Version: "versionname",
	}
	first.APIVersion, first.Kind = gvk.ToAPIVersionAndKind()

	second := first.DeepCopy()

	table := []struct {
		tname        string
		modify       func(*mutationsv1alpha1.AssignMetadata)
		areDifferent bool
	}{
		{
			"same",
			func(a *mutationsv1alpha1.AssignMetadata) {},
			false,
		},
		{
			"differentLocation",
			func(a *mutationsv1alpha1.AssignMetadata) {
				a.Spec.Location = "location"
			},
			true,
		},
		{
			"differentName",
			func(a *mutationsv1alpha1.AssignMetadata) {
				a.Name = "anothername"
			},
			true,
		},
		{
			"differentMatch",
			func(a *mutationsv1alpha1.AssignMetadata) {
				a.Spec.Match.Namespaces = []string{"foo", "bar"}
			},
			true,
		},
	}

	for _, tc := range table {
		t.Run(tc.tname, func(t *testing.T) {
			secondAssignMeta := second.DeepCopy()
			tc.modify(secondAssignMeta)
			firstMutator, err := mutation.MutatorForAssignMetadata(first)
			if err != nil {
				t.Fatal(tc.tname, "Failed to convert first to mutator", err)
			}
			secondMutator, err := mutation.MutatorForAssignMetadata(secondAssignMeta)
			if err != nil {
				t.Fatal(tc.tname, "Failed to convert second to mutator", err)
			}
			hasDiff := firstMutator.HasDiff(secondMutator)
			hasDiff1 := secondMutator.HasDiff(firstMutator)
			if hasDiff != hasDiff1 {
				t.Error(tc.tname, "Diff first from second is different from second to first")
			}
			if hasDiff != tc.areDifferent {
				t.Errorf("test %s: Expected to be different: %v, diff result is %v", tc.tname, tc.areDifferent, hasDiff)
			}
		})
	}
}
