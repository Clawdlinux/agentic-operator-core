# gdpr-eu v0.1.0

Engineering controls inspired by the EU General Data Protection Regulation.
This pack is NOT legal advice. It is NOT a certification. It does NOT make a
workload GDPR compliant. It covers a few checks only. Talk to your counsel.

Enable it with `spec.policyPacks: ["gdpr-eu@v0.1.0"]`.

Applies when the observed action carries `email`, `phone`, `iban`, or `card`.

| Rule | Effect | Fires when |
|---|---|---|
| GDPR-EU-01 | deny | `declaredIntent.purpose` is empty |
| GDPR-EU-02 | deny | the class is not in `declaredIntent.allowedDataClasses` |
| GDPR-EU-03 | require approval | `declaredIntent.decisionType` is `automated` (Art. 22 style) |

Detection is deterministic and has false positives, mostly for phone. See
[policy input](../../../../docs/policy-input.md).

This pack can only add deny or require approval. It cannot allow an action
that an invariant or another layer denied.
