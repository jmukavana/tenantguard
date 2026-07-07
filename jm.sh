#!/bin/bash

PROJECT="tenantguard"

#mkdir -p $PROJECT
#cd $PROJECT || exit

# Root files

touch LICENSE README.md CONTRIBUTING.md CODE_OF_CONDUCT.md 
ROADMAP.md SECURITY.md CHANGELOG.md GOVERNANCE.md ARCHITECTURE.md 
.gitignore .gitattributes .editorconfig

# GitHub

mkdir -p .github/workflows
mkdir -p .github/ISSUE_TEMPLATE

touch .github/workflows/{ci-java.yml,ci-go.yml,ci-python.yml,ci-typescript.yml,release.yml,security-scan.yml,docs.yml,codeql.yml}

touch .github/ISSUE_TEMPLATE/{bug_report.md,feature_request.md,question.md,architecture-proposal.md}

touch .github/PULL_REQUEST_TEMPLATE.md
touch .github/FUNDING.yml

# Docs

mkdir -p docs/{architecture,concepts,security,getting-started,integrations,adrs,diagrams/exports}

touch docs/{index.md,introduction.md,vision.md,roadmap.md,faq.md}

touch docs/architecture/{high-level-architecture.md,component-architecture.md,request-lifecycle.md,isolation-decision-tree.md,context-propagation.md,tenant-resolution-flow.md,event-flow.md,deployment-topologies.md}

touch docs/concepts/{tenant-context.md,isolation-strategies.md,tenant-resolvers.md,audit-hooks.md,compliance-hooks.md,distributed-tracing.md,policy-enforcement.md}

touch docs/security/{threat-model.md,compliance.md,gdpr.md,pci-dss.md,iso27001.md,auditforge-integration.md}

touch docs/getting-started/{installation.md,quickstart.md,hello-world-java.md,hello-world-go.md,hello-world-python.md,hello-world-typescript.md,demo-environment.md}

touch docs/integrations/{spring-boot.md,quarkus.md,micrometer.md,opentelemetry.md,kafka.md,redis.md,postgres.md,mongodb.md}

touch docs/adrs/{ADR-001-schema-isolation.md,ADR-002-w3c-trace-context.md,ADR-003-open-core.md,ADR-004-immutable-context.md,ADR-005-policy-engine.md,ADR-006-event-model.md}

touch docs/diagrams/{tenant-resolution.mmd,request-lifecycle.mmd,isolation-strategy.mmd,event-flow.mmd,deployment-topology.mmd}

# Core

mkdir -p core/{schemas,hooks}

touch core/{spec.md,TenantContext.md,TenantResolver.md,IsolationStrategy.md,AuditHook.md,PolicyHook.md,ContextPropagator.md}

touch core/schemas/{TenantEvent.schema.json,TenantContext.schema.json,AuditEvent.schema.json,ComplianceEvent.schema.json,PolicyDecision.schema.json}

touch core/hooks/{before-request.md,after-request.md,before-db-access.md,after-db-access.md,before-publish.md,before-audit.md}

# Java

mkdir -p java/{tenantguard-core,tenantguard-spring-boot-starter,tenantguard-jpa,tenantguard-jdbc,tenantguard-kafka,tenantguard-redis,tenantguard-auditforge,tenantguard-opentelemetry}

mkdir -p java/src/main/java/io/tenantguard
mkdir -p java/src/test/java
mkdir -p java/examples

touch java/pom.xml java/README.md

# Go

mkdir -p go/tenantguard/{context,resolver,isolation,propagation,audit,compliance,policy,telemetry,middleware}
mkdir -p go/examples

touch go/go.mod go/go.sum go/README.md

# Python

mkdir -p python/tenantguard/{context,resolver,isolation,propagation,audit,compliance,policy,telemetry,middleware}
mkdir -p python/examples

touch python/pyproject.toml python/README.md

# TypeScript

mkdir -p typescript/src/{context,resolver,isolation,propagation,audit,compliance,policy,telemetry,middleware}
mkdir -p typescript/examples

touch typescript/package.json typescript/tsconfig.json typescript/README.md

# SDK

mkdir -p sdk/{java-sdk,go-sdk,python-sdk,typescript-sdk}

# Examples

mkdir -p examples/spring-boot-example/{tenant-header-resolution,schema-isolation,audit-integration}
mkdir -p examples/{fastapi-example,gin-example,express-example,kafka-example,grpc-example,event-driven-saas}

# Benchmarks

mkdir -p benchmarks/{tenant-resolution,context-propagation,schema-switching,event-publishing}

# TestKit

mkdir -p testkit/{tenantguard-testcontainers,integration-tests,compliance-tests,performance-tests}

# Tools

mkdir -p tools/codegen/{generators,templates,cli}
mkdir -p tools/{migration,benchmarking,docs-generator}

# Platform

mkdir -p platform/docker
mkdir -p platform/kubernetes/{base,overlays,operators}
mkdir -p platform/terraform/{aws,gcp,azure}

touch platform/docker/{Dockerfile.java,Dockerfile.go,Dockerfile.python}

# Ecosystem

mkdir -p ecosystem/{auditforge,policyforge,tenantguard-ui,observability-pack,sample-reference-architecture}

echo "TenantGuard project structure generated successfully."

