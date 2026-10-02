package clawdlinux.decision_test

import data.clawdlinux.decision

full := {
	"declared": {"purpose": "kyc check", "decisionType": "assisted", "allowedDataClasses": ["aadhaar", "pan"]},
	"observed": {"dataClasses": ["aadhaar"]},
}

ids(set) := {f.id | some f in set}

test_no_pack_classes_no_findings if {
	inp := {"declared": {}, "observed": {"dataClasses": ["email"]}}
	count(decision.deny) == 0 with input as inp
	count(decision.require_approval) == 0 with input as inp
}

test_empty_input_no_findings if {
	count(decision.deny) == 0 with input as {}
}

test_declared_assisted_passes if {
	count(decision.deny) == 0 with input as full
	count(decision.require_approval) == 0 with input as full
}

test_missing_purpose_denied if {
	inp := json.patch(full, [{"op": "replace", "path": "/declared/purpose", "value": " "}])
	ids(decision.deny) == {"DPDP-IN-01"} with input as inp
}

test_missing_decision_type_denied if {
	inp := json.patch(full, [{"op": "remove", "path": "/declared/decisionType"}])
	ids(decision.deny) == {"DPDP-IN-02"} with input as inp
}

test_class_not_allowed_denied if {
	inp := json.patch(full, [{"op": "replace", "path": "/observed/dataClasses", "value": ["PAN"]}, {"op": "replace", "path": "/declared/allowedDataClasses", "value": ["aadhaar"]}])
	ids(decision.deny) == {"DPDP-IN-03"} with input as inp
}

test_undeclared_everything_denied if {
	inp := {"observed": {"dataClasses": ["aadhaar"]}}
	ids(decision.deny) == {"DPDP-IN-01", "DPDP-IN-02", "DPDP-IN-03"} with input as inp
}

test_automated_requires_approval if {
	inp := json.patch(full, [{"op": "replace", "path": "/declared/decisionType", "value": "Automated"}])
	ids(decision.require_approval) == {"DPDP-IN-04"} with input as inp
	count(decision.deny) == 0 with input as inp
}

test_automated_without_pack_classes_no_approval if {
	inp := {"declared": {"decisionType": "automated"}, "observed": {"dataClasses": ["email"]}}
	count(decision.require_approval) == 0 with input as inp
}
