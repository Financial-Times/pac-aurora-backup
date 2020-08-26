<!--
    Written in the format prescribed by https://github.com/Financial-Times/runbook.md.
    Any future edits should abide by this format.
-->

# PAC Aurora backup service

This service handles backups for AWS Aurora databases used by PAC (platform for annotation curation)

## Primary URL

<https://github.com/Financial-Times/pac-aurora-backup>

## Service Tier

Bronze

## Lifecycle Stage

Production

## Delivered By

content

## Supported By

content

## Known About By

- hristo.georgiev
- elina.kaneva
- mihail.mihaylov
- robert.marinov
- marina.chompalova

## Host Platform

AWS

## Architecture

Pac-aurora-backup is a cronjob deployed on PAC cluster and runs at schedule.

## Contains Personal Data

No

## Contains Sensitive Data

No

## Dependencies

- pac-aurora

## Failover Architecture Type

ActiveActive

## Failover Process Type

FullyAutomated

## Failback Process Type

FullyAutomated

## Failover Details

The cronjob is deployed in both Delivery clusters. The failover guide for the cluster is located here:
<https://github.com/Financial-Times/upp-docs/tree/master/failover-guides/delivery-cluster>

## Data Recovery Process Type

Manual

## Data Recovery Details

The backup can be restored from snapshot

## Release Process Type

PartiallyAutomated

## Rollback Process Type

Manual

## Release Details

The cronjob is deployed with Jenkins job. No failover is required as it is a Cronjob.

## Key Management Process Type

Manual

## Key Management Details

To access the job clients need to provide basic auth credentials to log into the k8s clusters.
To rotate credentials you need to login to a particular cluster and update varnish-auth secrets

## Monitoring

NotApplicable

## First Line Troubleshooting

<https://github.com/Financial-Times/upp-docs/tree/master/guides/ops/first-line-troubleshooting>

## Second Line Troubleshooting

Please refer to the GitHub repository README for troubleshooting information.
