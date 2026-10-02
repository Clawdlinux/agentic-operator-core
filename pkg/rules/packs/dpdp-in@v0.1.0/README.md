# dpdp-in v0.1.0

Engineering controls inspired by India's Digital Personal Data Protection Act
2023. This pack is NOT legal advice. It is NOT a certification. It does NOT
make a workload compliant with the DPDP Act. It covers a few checks only.
Talk to your counsel.

Enable it with `spec.policyPacks: ["dpdp-in@v0.1.0"]`.

Applies when the observed action carries `aadhaar` or `pan`.

| Rule | Effect | Fires when |
|---|---|---|
| DPDP-IN-01 | deny | `declaredIntent.purpose` is empty |
| DPDP-IN-02 | deny | `declaredIntent.decisionType` is empty |
| DPDP-IN-03 | deny | the class is not in `declaredIntent.allowedDataClasses` |
| DPDP-IN-04 | require approval | `declaredIntent.decisionType` is `automated` |

Detection is deterministic. Aadhaar needs a valid Verhoeff checksum. PAN is
pattern based. See [policy input](../../../../docs/policy-input.md).

This pack can only add deny or require approval. It cannot allow an action
that an invariant or another layer denied.
