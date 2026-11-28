---
title: "Sub-processors"
description: "The sub-processors Messora uses to provide its service: hosting, e-mail, AI and payment providers, where they are located and how changes are notified."
updated: 2026-11-28
version: "1.0"
---

This page lists the third-party service providers that process personal data while {{unvan}} ("**{{marka}}**") provides the Service. The list is published under clause 8 of the [Data Processing Agreement](/en/legal/data-processing-agreement).

This English version is provided for convenience; the Turkish version prevails in case of any discrepancy.

**Last updated:** 28 November 2026

## 1. Service infrastructure

| Sub-processor | Location | Service | Data processed | When |
|---|---|---|---|---|
| Amazon Web Services EMEA SARL | AWS eu-central-1, Frankfurt (Germany) | Hosting, database, file storage, encrypted backups | All data processed in the Service | Always |
| Resend, Inc. | United States of America | Transactional e-mails: e-mail verification, password reset, notifications, form replies | Recipient's name and e-mail address, e-mail content | Always |

## 2. Artificial intelligence providers

The following providers are engaged **only when the Customer enables artificial intelligence features and selects the relevant provider**, and only when that feature is used. The conversation content needed to generate a reply and the Customer's AI agent instructions are sent to the provider. Conversations are not used for model training.

| Sub-processor | Location | Service | Note |
|---|---|---|---|
| OpenAI, L.L.C. | United States of America | AI replies and information extraction | Requests are sent with `store: false` |
| Anthropic, PBC | United States of America | AI replies and information extraction | Under the commercial terms, API data are not used for training |
| Google LLC — Gemini API | United States of America | AI replies and information extraction | A key from a project with billing enabled (paid project) must be used |
| DeepSeek | Possibly the People's Republic of China | AI replies and information extraction | Only if the Customer selects it. Its servers may be in China; before using it for replies containing personal data, assess the provider's terms and the conditions for international transfers. |

Under the Enterprise Plan, if the Customer uses its own provider key, the contractual relationship with the provider is between the Customer and the provider, and the provider is not deemed a sub-processor of {{marka}}.

## 3. Payment

| Sub-processor | Location | Service | When |
|---|---|---|---|
| PayTR Ödeme ve Elektronik Para Kuruluşu A.Ş. | Türkiye | Card payment and refund transactions | Once card payment is introduced |

PayTR may also act as a data controller in its own right with respect to its own obligations under payment services legislation.

## 4. Parties based on the Customer's own agreement

| Party | Location | Service | Note |
|---|---|---|---|
| Meta Platforms Ireland Ltd. / WhatsApp | Ireland, the United States of America and other countries where Meta's data centres are located | Message delivery through the WhatsApp Business Cloud API | Within the scope of the Customer's own connected WhatsApp Business account and its agreement with Meta. Meta is not a sub-processor engaged by {{marka}}; it is listed for transparency. |

## 5. For website visitors only

| Provider | Location | Service | When |
|---|---|---|---|
| Google LLC — Google Analytics, Google Ads | United States of America | Usage and advertising conversion measurement on the {{web}} website | Only for visitors who consent to cookies. Not used in the panel or for Customer Data. Details: [Cookie Policy](/en/legal/cookie-policy) |

## 6. International transfers

Transfers to sub-processors located abroad are made, under Article 9 of Law No. 6698 on the Protection of Personal Data (KVKK), using the standard contracts announced by the Personal Data Protection Board (Kurul), and, where the GDPR applies, using the European Commission's standard contractual clauses. Details are set out in clause 10 of the [Data Processing Agreement](/en/legal/data-processing-agreement).

## 7. Notification of changes

- At least **30 days** before adding a new sub-processor or replacing an existing one, we inform our Customers at the Owner e-mail address or by panel notification, and we update this page.
- The Customer may object in writing to {{kvkkEposta}} within 30 days of the notification on reasonable grounds relating to data protection. The consequences of an objection are set out in clause 8.3 of the [Data Processing Agreement](/en/legal/data-processing-agreement).
- No prior notice is given for removing a sub-processor or narrowing the scope of data processed by an existing sub-processor; this page is updated.

For questions: {{kvkkEposta}}
