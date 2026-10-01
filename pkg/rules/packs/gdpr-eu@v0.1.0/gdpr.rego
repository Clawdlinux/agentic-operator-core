# gdpr-eu v0.1.0. Engineering controls inspired by the EU GDPR.
# Not legal advice. Not a certification. Not complete coverage of the GDPR.
package clawdlinux.decision

personal_classes := {"email", "phone", "iban", "card"}

observed := {lower(c) | some c in object.get(input, ["observed", "dataClasses"], [])}

allowed := {lower(trim_space(c)) | some c in object.get(input, ["declared", "allowedDataClasses"], [])}

present := observed & personal_classes

purpose := trim_space(object.get(input, ["declared", "purpose"], ""))

decision_type := lower(trim_space(object.get(input, ["declared", "decisionType"], "")))

deny contains {"id": "GDPR-EU-01", "reason": "personal data observed without a declared purpose"} if {
	count(present) > 0
	purpose == ""
}

deny contains {"id": "GDPR-EU-02", "reason": sprintf("%s observed but not in allowedDataClasses", [c])} if {
	some c in present
	not c in allowed
}

# Art. 22 style: a solely automated decision on personal data goes to a human.
require_approval contains {"id": "GDPR-EU-03", "reason": "automated decision on personal data needs human approval"} if {
	count(present) > 0
	decision_type == "automated"
}
