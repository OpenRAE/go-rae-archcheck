# Decision records

Each record states one design decision of the checker, its context, the options that it did not select and its consequences.
A new decision gets the next number, and a changed decision gets a new record that replaces the earlier one.
Design records of the model of the checker live in `architecture/designs`.

| Record | Decision |
| --- | --- |
| [0001](0001-one-model-for-structure-and-checks.md) | One model states the architecture and configures the checks. |
| [0002](0002-schema-validation.md) | The JSON schema validates the structure of each model. |
| [0003](0003-custom-mechanisms.md) | Four small mechanisms stay custom, because no maintained library fits them. |
