package clawdlinux.decision_test

import data.clawdlinux.decision

full := {
	"declared": {"purpose": "support replies", "decisionType": "assisted", "allowedDataClasses": ["email", "phone"]},
	"observed": {"dataClasses": ["email"]},
}

ids(set) := {f.id | some f in set}

test_no_personal_classes_no_findings if {
	inp := {"observed": {"dataClasses": ["aadhaar"]}}
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
	inp := json.patch(full, [{"op": "remove", "path": "/declared/purpose"}])
	ids(decision.deny) == {"GDPR-EU-01"} with input as inp
}

test_class_not_allowed_denied if {
	inp := json.patch(full, [{"op": "replace", "path": "/observed/dataClasses", "value": ["email", "IBAN"]}])
	ids(decision.deny) == {"GDPR-EU-02"} with input as inp
}

test_each_personal_class_checked if {
	every c in ["email", "phone", "iban", "card"] {
		ids(decision.deny) == {"GDPR-EU-01", "GDPR-EU-02"} with input as {"observed": {"dataClasses": [c]}}
	}
}

test_automated_requires_approval if {
	inp := json.patch(full, [{"op": "replace", "path": "/declared/decisionType", "value": "automated"}])
	ids(decision.require_approval) == {"GDPR-EU-03"} with input as inp
	count(decision.deny) == 0 with input as inp
}
