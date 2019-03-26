package target

##################
# Required Hooks #
##################

matching_constraints[constraint] {
	constraint := data["{{.ConstraintsRoot}}"][_][_]
	groups := {input.review.kind.group, "*"}
	spec := get_default(constraint, "spec", {})
	match := get_default(spec, "match", {})
	kindSelector := get_default(match, "kinds", [{"apiGroups": ["*"], "kinds": ["*"]}])
	selected_groups := {g | g = kindSelector[r].apiGroups[_]}
	remaining_groups := groups - selected_groups
	count(remaining_groups) < 2

	kinds := {input.review.kind.kind, "*"}
	selected_kinds := {k | k = kindSelector[r].kinds[_]}
	remaining_kinds := kinds - selected_kinds
	count(remaining_kinds) < 2

  labelSelector := get_default(match, "labelSelector", {})
	obj := get_default(input.review, "object", {})
	metadata := get_default(obj, "metadata", {})
	labels := get_default(metadata, "labels", {})
	matches_labelselector(labelSelector, labels)
}

# Namespace-scoped objects
matching_reviews_and_constraints[[review, constraint]] {
	obj = data["{{.DataRoot}}"].namespace[namespace][group][version][kind][name]
	r := make_review(obj, group, version, kind, name)
	review := add_field(r, "namespace", namespace)
	matching_constraints[constraint] with input as {"review": review}
}

# Cluster-scoped objects
matching_reviews_and_constraints[[review, constraint]] {
	obj = data["{{.DataRoot}}"].cluster[group][version][kind][name]
	review = make_review(obj, group, version, kind, name)
	matching_constraints[constraint] with input as {"review": review}
}

make_review(obj, group, version, kind, name) = review {
	review := {
		"kind": {"group": group, "version": version, "kind": kind},
		"name": name,
		"operation": "CREATE",
		"object": obj
	}
}

########
# Util #
########

add_field(object, key, value) = ret {
	keys := {k | object[k]}
	allKeys = keys | {key}
	ret := {k: v | v = get_default(object, k, value); allKeys[k]}
}

# has_field returns whether an object has a field
has_field(object, field) = true {
  object[field]
}

# False is a tricky special case, as false responses would create an undefined document unless
# they are explicitly tested for
has_field(object, field) = true {
  object[field] == false
}

has_field(object, field) = false {
  not object[field]
  not object[field] == false
}



# get_default returns the value of an object's field or the provided default value.
# It avoids creating an undefined state when trying to access an object attribute that does
# not exist
get_default(object, field, _default) = output {
  has_field(object, field)
  output = object[field]
}

get_default(object, field, _default) = output {
  has_field(object, field) == false
  output = _default
}


########################
# Label Selector Logic #
########################

# match_expression_violated checks to see if a match expression is violated.
match_expression_violated("In", labels, key, values) = true {
  has_field(labels, key) == false
}

match_expression_violated("In", labels, key, values) = true {
  # values array must be non-empty for rule to be valid
  count(values) > 0
  valueSet := {v | v = values[_]}
  count({labels[key]} - valueSet) != 0
}

# No need to check if labels has the key, because a missing key is automatic non-violation
match_expression_violated("NotIn", labels, key, values) = true {
  # values array must be non-empty for rule to be valid
  count(values) > 0
  valueSet := {v | v = values[_]}
  count({labels[key]} - valueSet) == 0
}

match_expression_violated("Exists", labels, key, values) = true {
  has_field(labels, key) == false
}

match_expression_violated("DoesNotExist", labels, key, values) = true {
  has_field(labels, key) == true
}


# Checks to see if a kubernetes LabelSelector matches a given set of labels
# A non-existent selector or labels should be represented by an empty object ("{}")
matches_labelselector(selector, labels) {
  keys := {key | labels[key]}
  matchLabels := get_default(selector, "matchLabels", {})
  satisfiedMatchLabels := {key | matchLabels[key] == labels[key]}
  count(satisfiedMatchLabels) == count(matchLabels)

  matchExpressions := get_default(selector, "matchExpressions", [])

  mismatches := {failure | failure = true; failure = match_expression_violated(
    matchExpressions[i]["operator"],
    labels,
    matchExpressions[i]["key"],
    get_default(matchExpressions[i], "values", []))}

  any(mismatches) == false
}