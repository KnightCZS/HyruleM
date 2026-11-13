# Synthetic EVFL fixtures

These files were generated from newly created, empty in-memory models with
`zeldamods/evfl` at commit `9e33dbadfc76516e7f87c5d8f4d98874b393e3e2`.
They contain no data extracted from a Nintendo game. The reference library is
a development-only format oracle and is not bundled with HyruleM.

- `synthetic-empty-quest.bfevfl`: V3 flow `SyntheticEmptyQuest`, zero actors,
  events and entry points; a valid empty DIC with its root node. Reproduces the
  parser assumption behind issue #1 without using the reported game file.
- `synthetic-entry-flow.bfevfl`: V3 flow `SyntheticMain`, entry points `Start`
  and `InternalHelper`, neither attached to an event.

Both were read back by the independent library before being added. Production
replacements continue to use an actor and a terminating action; an empty input
flow does not cause HyruleM to generate a replacement with zero entry points.
