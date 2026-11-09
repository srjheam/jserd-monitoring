# JSERD Monitoring

A system that collects the editorial state of the JSERD journal from Open
Journal Systems, and presents it to the chief editors for curation. It exists to
support the TCC case study on monitoring automation for journal curation.

## Language

### Deployment stages

**Validation stage**:
The stage in which the system collects live JSERD data and chief editors curate
it, but the system sends no e-mail. Every message goes to the intercepting
transport.
_Always write both words._ Bare "validation" means input checking in this
codebase, as in `config.Validate`.

**Delivery stage**:
The stage in which the system sends real e-mail to real chief editors. Curation
works the same as in the validation stage. E-mail is the only difference.
_Avoid_: production, live mode, stage two

### Editorial work

**Curation**:
The work a chief editor performs on a submission in the dashboard: a note, a
conference association, a mute, or an up-to-date mark. Curation produces original data
that no collection can rebuild.
_Avoid_: review, triage, annotation
_Note_: an attention category and a verification were curation until the server
took ownership of both. See **Finding**, **Muted** and **Up to date**. A local
final state and a local decision date were curation too, until the dashboard
rework removed both without migrating the old values. Current OJS status
supplies the final outcome. **Decisions** preserve the editorial history.

**Chief editor**:
A person responsible for the editorial state of JSERD submissions, using a
dedicated account meant for that work alone. The dashboard accepts sign-in
from a Chief editor account only, never from a Superuser account, even for
the same real person.
_Avoid_: editor, user, operator, admin

**Superuser**:
A person who holds `_superusers` access: the PocketBase admin panel, reachable
only through an SSH tunnel. There is one Superuser in the system. A Superuser
does not sign in to the dashboard; dashboard curation always happens through a
separate Chief editor account, even for the same real person.
_Avoid_: admin, operator, editor
_Note_: this is PocketBase's own term for the role (`_superusers` collection,
the `superuser` CLI command). Do not use "admin" or "operator" even though
both appeared earlier in project discussion.

**Finding**:
One computed statement that a submission needs a chief editor. The server
derives every finding from collected OJS evidence. A chief editor never writes
one.
_Avoid_: alert, issue, problem, attention category
_Note_: never use "finding" for the output of a linter, a formatter, or a
test. That output is a **violation**.

**Pending work**:
The set of open findings on a submission. A submission has pending work when it
carries at least one finding.
_Avoid_: backlog, todo

**Muted**:
The condition of a submission that a chief editor chose to keep off the work
list, because a conference owns it and the assigned editor is trusted.
The system keeps collecting and keeps computing findings for a muted
submission. Only the chief editor stops seeing it. Muting ends when a chief
editor unmutes, or when the submission reaches a final state.
_Avoid_: untracked, unmonitored, ignored, archived, hidden
_Note_: never say that the system untracks a muted submission. It does not.

**Up to date**:
The condition of a submission that needs nothing from a chief editor now. A
submission is up to date when it carries no finding, or when a chief editor set
it aside and its findings have not changed since.
_Avoid_: snoozed, deferred, done, dormant

**Finding digest**:
A hash of the finding kinds on a submission, with no deadlines and no counts
inside it. Two collections that find the same problems produce the same digest.
The digest decides when a submission stops being up to date.
_Avoid_: fingerprint, signature, checksum
_Note_: do not confuse with the **Editorial summary e-mail**, which chief
editors also used to call a digest.

**Cycle**:
The one recurring unit of work: collect from OJS, process the changes,
compute findings, recompute the dashboard, and create pending notifications.
A cycle runs hourly or on demand, from a manual collection request. There is
no separate weekly pass: every cycle does the same steps.
_Avoid_: collection cycle, editorial run, weekly run
_Note_: `collection_cycles` is only the raw OJS fetch and snapshot inside a
cycle, kept as the name of that database table. The cycle is the whole thing.

**Editorial summary e-mail**:
The e-mail a cycle sends to chief editors, reporting the findings that stand
right now, as of the moment it was sent. It carries no date range: findings
are live, so there is no period for it to summarize.
_Avoid_: digest, weekly digest, editorial digest

**Conference**:
A conference whose submissions are grouped for curation in JSERD Monitoring.
A submission belongs to at most one Conference.
_Avoid_: editorial event, submission group, collection, track, special issue, OJS section
_Note_: a conference association belongs to JSERD Monitoring alone. OJS does
not provide it. Assigning or changing a Conference mutes the submission.

**Unassign**:
The chief editor action that clears a submission's Conference and unmutes
the submission in the same step. A conference association ends only through
this explicit action, never on its own.
_Avoid_: remove conference, clear conference, detach

**Assigned editor**:
An OJS staff member named on a submission, collected from OJS as a given name
and a family name only. OJS gives no id for this person, so two different
assigned editors with the same name cannot be told apart by the system.
_Avoid_: editor, user, operator
_Note_: never confuse with **Chief editor**, the dashboard's own user. An
assigned editor is never a chief editor and never uses the dashboard. See
**Recipient** for how the system merges an assigned editor's identity with a
reviewer's for e-mail purposes.

**Reviewer**:
An OJS peer reviewer assigned to a submission, collected from OJS with a
stable username. Unlike an **Assigned editor**, a reviewer's identity never
collides with another reviewer's, since OJS's review export carries the
username.
_Avoid_: referee

**Copyeditor**:
An OJS participant assigned through the Copyeditor group in a submission's
Copyediting stage. A generic assigned editor does not prove this assignment.
The `missing_copyeditor` finding requires observed absence of that assignment.
RN18 notifies the submission's assigned editors when the absence first appears.
Active Copyediting belongs to Archived only after a confirmed Copyeditor assignment.
An absent or unknown assignment follows the existing curation rules.
Rows retain the Copyediting tag in every queue. An acceptance decision does not make a submission final.

**Recipient**:
A person the system can send e-mail to: an **Assigned editor** or a
**Reviewer** with a valid e-mail address on file. An assigned editor and a
reviewer that share the same name-slug are treated as the same Recipient, on
the same terms ADR 0006 already accepts for two assigned editors who share a
name.
_Avoid_: user, contact, recipient record
_Note_: an assigned editor or a reviewer with no e-mail address on file is
never a Recipient. The dashboard still shows and searches them by name;
only e-mail delivery ignores them.

**Recipient role**:
The Assigned editor or Reviewer role for which a person receives a particular
notification. A person can hold both roles, but each notification targets one role.

**Welcome e-mail**:
The one-time e-mail a Recipient gets before their first triggered
notification e-mail. The system sends it once per Recipient, never again.
_Avoid_: onboarding e-mail, greeting
_Note_: this applies only to the RN1-RN8 and RN18 notification pipeline, never to the
Chief editor password-reset e-mail, which is a separate, pre-existing mail
path (see ADR 0011).

**Work**:
The queue holding a submission that carries **Pending work**, was not set
aside as **Up to date**, is not **Muted**, and is not in a final state.
_Avoid_: active, worklist, inbox

**Queue**:
The one place a submission sits. There are four, in this order: Work (stored
as `daily`), **Up to date**, **Muted**, Archived. A submission is in exactly
one at a time, and the server decides which. The order is the display order.
Archived holds Published, Declined or Rejected, and Withdrawn submissions, plus active
Copyediting submissions with a confirmed Copyeditor assignment.
Current OJS status determines final outcomes. Decision history does not.
_Avoid_: folder, screen, bucket, state
_Note_: the client calls the Work queue `work` in a URL and a `folder:` chip,
and the database calls it `daily`. Those are two spellings of this one queue.
A **Side screen** is the page that shows one of the other three; a queue is
the placement, and a side screen is the view of it.
