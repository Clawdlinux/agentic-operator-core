# dpdp-in v0.1.0. Engineering controls inspired by India's DPDP Act 2023.
# Not legal advice. Not a certification. Not complete coverage of the Act.
package clawdlinux.decision

pack_classes := {"aadhaar", "pan"}

observed := {lower(c) | some c in object.get(input, ["observed", "dataClasses"], [])}

allowed := {lower(trim_space(c)) | some c in object.get(input, ["declared", "allowedDataClasses"], [])}

present := observed & pack_classes

purpose := trim_space(object.get(input, ["declared", "purpose"], ""))

decision_type := lower(trim_space(object.get(input, ["declared", "decisionType"], "")))

deny contains {"id": "DPDP-IN-01", "reason": "aadhaar or pan observed without a declared purpose"} if {
	count(present) > 0
	purpose == ""
}

deny contains {"id": "DPDP-IN-02", "reason": "aadhaar or pan observed without a declared decisionType"} if {
	count(present) > 0
	decision_type == ""
}

deny contains {"id": "DPDP-IN-03", "reason": sprintf("%s observed but not in allowedDataClasses", [c])} if {
	some c in present
	not c in allowed
}

require_approval contains {"id": "DPDP-IN-04", "reason": "automated decision with aadhaar or pan needs human approval"} if {
	count(present) > 0
	decision_type == "automated"
}
