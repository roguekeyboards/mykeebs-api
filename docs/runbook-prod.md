# Prod runbook

What to do when something goes wrong in production, and how to do the rare jobs that only happen in prod.

**This is a living document.** Whenever prod surprises us, add how to handle it next time: a new section if it's a new kind of problem, or a step in the section it belongs to. A procedure that turned out to be wrong gets fixed here, not just in someone's memory.

## What prod is

| | |
|---|---|
| API | `https://api.mykeebs.info` |
| AWS account | `mykeebs-api-prod` (`416860882648`), region `us-east-2` |
| Stack | `mykeebs-api-prod`, defined by the `[prod]` section of `samconfig.toml` |
| Identity provider | the Stytch **Live** project, issuer `https://auth.mykeebs.info` |
| Alarm emails | `mykeebs-alerts-prd@mykeebs.com` |
| Budget emails | `mykeebs-budget-prd@mykeebs.com` |

Prod is deployed only by `deploy-prod.yml`, for every published release.

## Before you start

Everything below that touches AWS needs the prod admin profile:

```sh
aws sso login --sso-session mykeebs-api
export AWS_PROFILE=mykeebs-api-prod-admin AWS_REGION=us-east-2
```

## When an alarm email arrives

An alarm that's already back to OK still means something failed.

To see what the API function logged around a time (change the start and end to bracket the alarm's timestamp, which is in UTC):

```sh
START=$(date -j -u -f %Y-%m-%dT%H:%M:%S 2026-10-19T18:10:00 +%s)000
END=$(date -j -u -f %Y-%m-%dT%H:%M:%S 2026-10-19T18:20:00 +%s)000
aws logs filter-log-events --log-group-name /aws/lambda/mykeebs-api-prod \
  --start-time $START --end-time $END --output json |
  jq -r '.events[] | "\(.timestamp/1000 | strftime("%H:%M:%S")) \(.message)"'
```

Use `/aws/lambda/mykeebs-api-prod-moderate` for the moderation function.

| Alarm | What it means | Where to start |
|---|---|---|
| `ApiFunctionErrorsAlarm` | The API function crashed or timed out on a request. Normal error responses (4xx, or a 5xx the code returns on purpose) don't count. | Search the logs for `ERROR` and `Status: timeout`. |
| `HttpApi5xxAlarm` | API Gateway returned a 5xx to a caller. It usually fires together with the one above. | Same as above. If it fires alone, the function is returning a 5xx itself: look for `"status":5` in the logs. |
| `ModerateFunctionErrorsAlarm` | The moderation function failed on an upload. Lambda retries it. | The moderation function's logs. If retries keep failing, the event ends up in the dead-letter queue (next row). |
| `ModerationDLQAlarm` | An upload failed moderation on every retry, so the image stays pending. | [Failed images](moderation.md#failed-images) in the moderation guide: list the queue, fix the cause, redrive. |
| `AccountDeleteNoticeFailedAlarm` | Someone deleted their account, but the notice below didn't send. | Search the API logs for `account delete notice failed`; the line's `owner_id` is the user to remove. Then follow [When an account is deleted](#when-an-account-is-deleted). The logs are kept 14 days. |

## When an account is deleted

Deleting a profile deletes the whole account in mykeebs-api, but not the user's sign-in record at Stytch (Discord ID, username, email): mykeebs-api holds no Stytch secret. Each delete emails the alarm address a notice, "mykeebs-api: account deleted, remove the user from the identity provider", naming the user ID (`user-live-…`). The privacy policy promises removal within 30 days.

1. In the Stytch dashboard for the Live project, open **Users** and search for the user ID.
2. Delete the user.

If the user signs in again later, Stytch creates a new user with a new ID, so they start with an empty account.

## When someone may have got at user data

This covers anyone outside the project reading or changing data they shouldn't: a leaked AWS or Stytch credential, a bug that served one user's private items to another, a breach reported by AWS, Stytch or Cloudflare. Discord's developer terms require prompt notice to Discord, and to users where the law requires it.

1. Stop it first: revoke the leaked credential, or stop API traffic (see [When a budget email arrives](#when-a-budget-email-arrives)).
2. Work out what was reachable, whose, and when, from the API logs (kept 14 days) and CloudTrail. Write it down as you go.
3. Tell Discord through [Discord developer support](https://support-dev.discord.com/hc/en-us/requests/new): what happened, when, which data, and what you've done. Do it within days, not after the investigation is finished.
4. Email the affected users, at the email Stytch has for them, if their private data (email, private items, photos) was exposed. GDPR's deadline to tell the regulator is 72 hours; if EU or UK users are affected, check whether that applies.
5. Add what happened and how to stop it recurring to this runbook.

## When a request about someone's data arrives

The privacy policy promises an answer within 30 days. Check the request comes from the email on the account (the Stytch Live dashboard shows it under **Users**) before acting on it.

- **A copy:** [Exporting a user's data on request](#exporting-a-users-data-on-request). Zip the dump and send it.
- **A correction** they can't make themselves: the same impersonation, then edit through the API or an MCP client.
- **Deletion:** ask them to delete their account from the site; if they can't, impersonate them and delete the profile, then follow [When an account is deleted](#when-an-account-is-deleted).
- **Reports of someone breaking the rules:** look at what was reported, and remove it or suspend the account if it breaks the terms. Reply to say what you did.

## When a budget email arrives

The budget is $10 a month, emailing at 80% and 100%. Prod normally costs pennies.

1. **Find what's costing money:** Cost Explorer in the `mykeebs-api-prod` console, grouped by service, daily.
2. **If it's Rekognition**: pause moderation (`mise run moderation-pause prod`; see the [moderation guide](moderation.md#pausing-moderation)).
3. **If it's CloudFront data transfer**: use the image switch below.
4. **If you need to stop spending right now**, use one of the emergency switches below. Both are made outside the template: undo them by hand, and check they're still in place after any deploy.

Emergency switches:

```sh
# Stop all API traffic: every request fails until you undo it.
aws lambda put-function-concurrency --function-name mykeebs-api-prod \
  --reserved-concurrent-executions 0
# Undo (15 is ApiReservedConcurrency in samconfig.toml's [prod] section):
aws lambda put-function-concurrency --function-name mykeebs-api-prod \
  --reserved-concurrent-executions 15
```

That stops new image links being issued too, but links already handed out keep working until they expire (up to 24 hours). To stop those immediately, disable the images distribution:

```sh
DIST=$(aws cloudformation describe-stacks --stack-name mykeebs-api-prod \
  --query "Stacks[0].Outputs[?OutputKey=='ImagesDistributionId'].OutputValue" --output text)
aws cloudfront get-distribution-config --id "$DIST" > dist.json
jq '.DistributionConfig.Enabled = false | .DistributionConfig' dist.json > disabled.json
aws cloudfront update-distribution --id "$DIST" --if-match "$(jq -r .ETag dist.json)" \
  --distribution-config file://disabled.json
# Undo: the same with Enabled = true. The next deploy also re-enables it.
```

## Releases and deploys

### How a change reaches prod

1. A PR merges to `main`.
2. `release.yml` cuts a release if the merge has a `fix` or `feat` commit.
3. The release starts `deploy-prod.yml`, which deploys it and smoke-tests the API.

### A release broke something

Never roll back. Merge a fix and let it release.

### A deploy failed

Open the failed `Deploy Prod` run in GitHub Actions.

- **If it failed for a transient reason** (an AWS hiccup, a timeout pulling an image, a smoke test that hit a cold start), click **Re-run failed jobs**.
- **If it failed because of the change itself**, CloudFormation has already rolled the stack back to the previous version, so prod is still serving. Fix forward with a new release.
- **If the stack is stuck**, see the next section.

### The stack is stuck in `UPDATE_ROLLBACK_FAILED`

A failed release that added something the deploy role can't delete (a table, say) leaves the stack here. Prod keeps serving, but nothing deploys until it's fixed.

1. Find the resource that couldn't be rolled back:
   ```sh
   aws cloudformation describe-stack-events --stack-name mykeebs-api-prod \
     --query "StackEvents[?ResourceStatus=='DELETE_FAILED'].[LogicalResourceId,PhysicalResourceId,ResourceStatusReason]" \
     --output table
   ```
2. Tell CloudFormation to skip it and finish the rollback:
   ```sh
   aws cloudformation continue-update-rollback --stack-name mykeebs-api-prod \
     --resources-to-skip <LogicalResourceId>
   aws cloudformation wait stack-rollback-complete --stack-name mykeebs-api-prod
   ```
3. Delete the leftover resource by hand (its name is the `PhysicalResourceId` from step 1). It isn't part of the stack anymore.
4. Re-run the failed deploy, or merge a fix.

### A merge didn't produce a release

If there's no `Release` run for the merge commit in the Actions tab:

```sh
gh workflow run release.yml
```

## Data

The user-data tables have deletion protection and point-in-time recovery (PITR) on. The images bucket has no versioning, so a deleted image is gone.

### Restoring a table to an earlier point in time

PITR restores a table as it was at any second in the last 35 days, but only into a **new** table, so you copy the data back from it.

1. Find the live table's name (`SwitchTableName`, `KeyboardTableName`, `KeycapSetTableName`, `BuildTableName`, `ProfileTableName` or `ProfileUsernameTableName`):
   ```sh
   LIVE=$(aws cloudformation describe-stacks --stack-name mykeebs-api-prod \
     --query "Stacks[0].Outputs[?OutputKey=='KeyboardTableName'].OutputValue" --output text)
   ```
2. Restore it as a new table, at a time in UTC, then wait:
   ```sh
   aws dynamodb restore-table-to-point-in-time --source-table-name "$LIVE" \
     --target-table-name "$LIVE-restore" --restore-date-time 2026-10-19T18:00:00Z
   aws dynamodb wait table-exists --table-name "$LIVE-restore"
   ```
3. Look at the restored table (in the console, or with `aws dynamodb scan`) and make sure it's what you want.
4. Copy its items back into the live table. This overwrites items with the same key and leaves everything else alone:
   ```sh
   aws dynamodb scan --table-name "$LIVE-restore" --output json | jq -c '.Items[]' |
     while read -r item; do
       aws dynamodb put-item --table-name "$LIVE" --item "$item"
     done
   ```
   If items created after the restore point should go too, delete them from the live table by key. Do that one by one, deliberately: there's no undo.
5. Check through the API (or MCP) that the data looks right, then delete the restored table:
   ```sh
   aws dynamodb delete-table --table-name "$LIVE-restore"
   ```

Images aren't restored: an item that points to a since-deleted image shows it as missing.

### Backing up images

A `mykeebs-migrate dump` (next section) is the only backup of an account's images.

### Running `mykeebs-migrate` against prod

`dump --impersonate` works against prod because the Stytch Live project has `http://localhost:8765/impersonate` as its default Login redirect URL and `http://localhost:8765` as an authorized SDK domain. Don't remove them. `mykeebs-migrate login` needs neither: its redirect is registered on the OAuth client it creates.

```sh
mise run migrate-build
bin/mykeebs-migrate login --issuer https://auth.mykeebs.info
bin/mykeebs-migrate dump --issuer https://auth.mykeebs.info --base-url https://api.mykeebs.info --out ./prod-dump
```

After a `restore`, run `verify`. See [`mykeebs-migrate`](../CONTRIBUTING.md#backing-up--moving-a-users-data-mykeebs-migrate) for the full tool.

### Exporting a user's data on request

Use `dump --impersonate` to dump their account; see [Exporting another user's account](../CONTRIBUTING.md#exporting-another-users-account). Turn **Allow user impersonation** on in the Stytch Live dashboard only for the export, and off again straight after.

## Decommissioning prod

`mise run teardown prod` refuses on purpose. Taking prod down is a manual job, with the admin profile. The terms promise users 30 days' notice first, so they can save their data, and Discord's developer terms require deleting everything received through Discord once the app stops.

1. Take a `mykeebs-migrate dump` of every account whose data you want to keep. Don't keep those dumps once they've been restored somewhere else.
2. Turn off deletion protection on every table:
   ```sh
   for t in $(aws dynamodb list-tables --query 'TableNames[?starts_with(@, `mykeebs-api-prod-`)]' --output text); do
     aws dynamodb update-table --table-name "$t" --no-deletion-protection-enabled >/dev/null
   done
   ```
3. Empty the images bucket, then delete the stack:
   ```sh
   aws s3 rm s3://mykeebs-images-416860882648-mykeebs-api-prod --recursive
   aws cloudformation delete-stack --stack-name mykeebs-api-prod
   aws cloudformation wait stack-delete-complete --stack-name mykeebs-api-prod
   ```
4. Delete the image repository and its stack (the repository is kept when its stack is deleted):
   ```sh
   aws cloudformation delete-stack --stack-name mykeebs-api-ecr-bootstrap-mykeebs-api-prod
   aws ecr delete-repository --repository-name mykeebs-api-prod --force
   ```
5. Stop the GitHub deploys by deleting the `mykeebs-api-prod-oidc` stack, and delete the `api.mykeebs.info` CNAME in Cloudflare.
6. Delete the Stytch Live project, which holds everyone's Discord ID, name, avatar and email. Then delete the prod application in the Discord developer portal.
7. The remaining bootstrap stacks (`mykeebs-api-bootstrap`, the artifact bucket, and `mykeebs-api-stack-boundary`) cost nothing to keep. Delete them only if the account itself is going away.

