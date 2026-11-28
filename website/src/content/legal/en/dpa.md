---
title: "Data Processing Agreement"
description: "How Messora processes customer data as a data processor: instructions, security measures, sub-processors and breach notification, under KVKK and the GDPR."
updated: 2026-11-28
version: "1.0"
---

This Data Processing Agreement (the "**DPA**") forms an integral part of the {{marka}} [Terms of Service and Service Agreement](/en/legal/terms-of-service) (the "**Main Agreement**") and enters into force upon acceptance of the Main Agreement. The DPA governs the parties' obligations under Article 12/2 of Law No. 6698 on the Protection of Personal Data ("**KVKK**") and, to the extent applicable, Article 28 of the General Data Protection Regulation (EU) 2016/679 ("**GDPR**").

This English version is provided for convenience; the Turkish version prevails in case of any discrepancy.

## 1. Parties and roles

1.1. **Data controller:** The customer accepting the Main Agreement (the "**Customer**").

1.2. **Data processor:** {{unvan}} ("**{{marka}}**"); MERSIS No. (Central Registration System number): {{mersis}}; address: {{adres}}; KEP (registered electronic mail): {{kep}}; e-mail: {{kvkkEposta}}.

1.3. With respect to personal data that the Customer uploads to the panel or processes through the Service (the "**Customer Personal Data**"), the Customer is the data controller and {{marka}} is the data processor. Such data include in particular: contact records, telephone numbers, WhatsApp messages and media files, notes, tags, consent records, campaign, template, automation and AI agent data.

1.4. With respect to the account, contact, invoicing, payment and security data of Customer representatives and Users, {{marka}} is the **data controller**; such data are outside the scope of the DPA and are subject to the [Privacy Notice (KVKK)](/en/legal/privacy-notice) and the [Privacy Policy](/en/legal/privacy-policy).

## 2. Definitions

The terms "personal data", "special categories of personal data", "processing", "data subject", "data controller", "data processor" and "explicit consent" used in the DPA have the same meaning as in the KVKK and, where the GDPR applies, as their equivalents in the GDPR. "**Sub-processor**" means a third-party data processor engaged by {{marka}} to process Customer Personal Data. "**Breach**" means a security incident resulting in Customer Personal Data being obtained by others by unlawful means, altered, lost or accessed without authorisation. Capitalised terms not defined in the DPA have the meanings given to them in the Main Agreement.

## 3. Subject matter, nature, purpose and duration of processing

| | |
|---|---|
| Subject matter | Provision of the Service to the Customer |
| Nature | Hosting, recording, storing, organising, classifying, transmitting to and receiving from the WhatsApp Business Cloud API, displaying, reporting, exporting, backing up and deleting; transmitting to an artificial intelligence provider when enabled by the Customer |
| Purpose | Enabling the Customer to communicate with its own customers over WhatsApp using the features of the Service (team inbox, campaigns, templates, automations, AI agents, analytics, API and webhooks) |
| Duration | The term of the Main Agreement and the return and deletion periods specified in clause 14 |

## 4. Categories of personal data and data subjects

4.1. **Data subjects:** The Customer's customers and prospective customers, other persons with whom the Customer corresponds over WhatsApp, and business partners and employees recorded in the Service by the Customer.

4.2. **Data categories:** Identity (first name, last name, WhatsApp profile name); contact (telephone number, e-mail, city and other fields defined by the Customer); customer transaction (message contents, media, order and request information, notes, tags, segments); marketing (consent and opt-out records, campaign interactions, delivery, read and reply status); information collected by the Customer through the AI agent; other data contained in lists uploaded by the Customer.

4.3. **Special categories of personal data:** The Service is not designed for processing special categories of personal data (such as health, sex life, religion, sect, political opinion, criminal convictions, biometric and genetic data). The Customer should not upload such data to the Service and should not request them in WhatsApp correspondence. A Customer operating in sectors such as health, education, law or finance, where data subjects may share such data on their own initiative, is responsible for taking the adequate measures required by KVKK Art. 6 and by the Personal Data Protection Board (Kurul) for special categories of personal data, for establishing the necessary legal ground, and for assessing the use of artificial intelligence features with such data.

## 5. Obligations of the Customer

The Customer accepts and undertakes that:

- it has collected the Customer Personal Data lawfully, has informed data subjects in accordance with KVKK Art. 10 and has obtained explicit consent where required,
- it has obtained the consents (opt-in) required by Meta policies for WhatsApp messages and the approvals required under Law No. 6563,
- it has established the legal grounds and safeguards required by KVKK Art. 9 with respect to international transfers (in particular under clauses 8 and 10),
- it will respond to data subject requests, and
- the instructions it gives to {{marka}} comply with the legislation.

If the Customer is subject to the obligation to register with VERBİS (Data Controllers' Registry Information System), it fulfils the registration obligation itself.

## 6. Obligations of {{marka}}

### 6.1. Instructions

{{marka}} processes Customer Personal Data only on the Customer's documented instructions. The Main Agreement, the DPA, the settings and actions the Customer performs in the panel (for example, launching a campaign, setting up an automation, selecting an AI provider, deleting and exporting data) and calls made through the API are deemed the Customer's instructions. If {{marka}} considers that an instruction infringes the legislation, it informs the Customer without delay. Where it is legally required to carry out processing, it notifies the Customer of that requirement before processing, unless the law prohibits this.

### 6.2. Confidentiality

{{marka}} limits access to Customer Personal Data to employees for whom it is necessary for providing the Service, support and security. These persons are subject to written confidentiality obligations, which continue after their duties end. {{marka}} employees access Customer Personal Data for support purposes only where necessary, such as at the Customer's request or to investigate a security incident.

### 6.3. Purpose limitation

{{marka}} does not use Customer Personal Data for its own purposes, does not sell them, does not share them with third parties for marketing purposes and does not use them to train artificial intelligence models. Only anonymous and aggregated statistics that do not contain personal data may be used to improve the Service.

## 7. Technical and organisational measures

Taking into account KVKK Art. 12 and the Personal Data Security Guide (Kişisel Veri Güvenliği Rehberi) of the Personal Data Protection Board, {{marka}} applies the following measures appropriate to the nature and risks of the processing:

**Hosting and infrastructure**

- The Service is hosted in the Amazon Web Services Frankfurt (Germany, eu-central-1) region.
- The database, cache and real-time communication components are not directly exposed to the internet; external access is only through a reverse proxy over HTTPS.
- Application containers run as an unprivileged user, with a read-only file system and with unnecessary privileges removed.
- Data are backed up in encrypted form every night; backups are renewed on a cycle of at most 35 days.

**Data separation and access control**

- Data separation between Customers is ensured by PostgreSQL row-level security; every record is tagged with the company to which it belongs.
- Role-based authorisation (Owner, Admin, Manager, Agent) and per-module permissions are applied; depending on the setting, the Agent role sees only conversations assigned to it.
- Every identifier (contact, group, template, number) is verified for authorisation at the service layer.

**Authentication**

- Passwords are hashed with Argon2id; a minimum of 12 characters and a common-password check are enforced.
- Two-factor authentication (TOTP and single-use recovery codes) is supported; it can be made mandatory for Owners and Admins.
- Rate limiting is applied to login attempts; the account is temporarily locked after repeated failed attempts.
- Sessions expire after a set period of inactivity and an absolute duration; other sessions are closed when the password is changed.
- Sensitive operations such as creating an API key, changing the Meta access token or changing a role require re-authentication.

**Encryption and secrets**

- Data in transit are encrypted with HTTPS (TLS).
- Meta access tokens, AI provider keys and similar secrets are stored encrypted with AES-256-GCM; only their last characters are shown in the interface. API keys are kept as one-way hashes.
- Message contents and telephone numbers are not written to application logs; secrets are masked in logs.

**Logging and monitoring**

- Operations such as logins, permission changes, API keys, Meta accounts, campaigns, imports and exports, contact deletion and consent changes are written to an audit log that cannot be changed or deleted through the application.

**Application security**

- The signatures of incoming Meta webhooks are verified; outgoing webhooks are signed with HMAC-SHA256.
- Access to the server's internal network (SSRF) is blocked in automation and webhook requests.
- File uploads are checked for size, extension and content type; measures are taken against formula injection in exported spreadsheets.
- Dependencies are pinned and vulnerability scanning is carried out in continuous integration.

**Messaging compliance**

- Sending marketing messages to persons who have opted out is blocked at system level; free-form text is not sent outside the 24-hour window; campaigns are stopped automatically on quality warnings.

{{marka}} may update these measures in line with technological developments; updates are made in a way that does not lower the level of protection.

## 8. Sub-processors

8.1. The Customer gives {{marka}} general authorisation to engage sub-processors to provide the Service. The current list is published on the [Sub-processors](/en/legal/subprocessors) page.

8.2. At least **30 days** before adding a new sub-processor or replacing an existing one, {{marka}} informs the Customer by e-mail or panel notification and updates the list.

8.3. The Customer may object in writing within 30 days of the notification on reasonable grounds relating to data protection. The parties seek a reasonable solution; if no solution is found, the Customer may terminate the contract in respect of the part of the Service that requires the use of the sub-processor objected to, and the fee for the paid but unused period is refunded.

8.4. {{marka}} enters into written agreements with sub-processors containing data protection obligations no less protective than those in this DPA and is liable to the Customer for the acts of its sub-processors.

8.5. **Artificial intelligence providers** (OpenAI, Anthropic, Google Gemini, DeepSeek) are engaged as sub-processors only when the Customer enables artificial intelligence features and selects the relevant provider, and only for that use. Where the Customer uses its own provider key (Enterprise Plan), the contractual relationship with the relevant provider is between the Customer and the provider, and the provider is not deemed a sub-processor of {{marka}}.

## 9. Position of Meta

9.1. WhatsApp messages are delivered by Meta Platforms Ireland Ltd. and its affiliates ("**Meta**") through the WhatsApp Business Cloud API. The Customer connects its own Meta Business and WhatsApp Business account to the Service and accepts the WhatsApp Business terms directly with Meta.

9.2. Meta processes message data under the WhatsApp Business terms between the Customer and Meta and Meta's data processing terms. Meta is not a sub-processor engaged by {{marka}}; the relationship with Meta is based on the Customer's own agreement. On the Customer's instructions, {{marka}} transmits messages to Meta and receives messages from Meta. {{marka}} is not responsible for how Meta processes such data, the location of Meta's data centres or Meta's terms. For transparency, Meta is also listed on the [Sub-processors](/en/legal/subprocessors) page.

## 10. International transfers

10.1. Customer Personal Data are processed in the AWS Frankfurt region where the Service is hosted and in the countries of the sub-processors listed on the [Sub-processors](/en/legal/subprocessors) page.

10.2. Transfers from Türkiye abroad are made under KVKK Art. 9, as amended by Law No. 7499, and the Regulation on the Procedures and Principles for the Transfer of Personal Data Abroad (Kişisel Verilerin Yurt Dışına Aktarılmasına İlişkin Usul ve Esaslar Hakkında Yönetmelik). For transfers to countries without an adequacy decision, the **standard contracts** announced by the Personal Data Protection Board are used: the controller-to-processor transfer module between the Customer and {{marka}}, and the processor-to-processor transfer module between {{marka}} and its sub-processors. The standard contracts are notified by the relevant party to the Personal Data Protection Authority (Kişisel Verileri Koruma Kurumu) within five business days of signature. The parties cooperate with each other in signing and notifying these contracts.

10.3. For transfers subject to the GDPR, the standard contractual clauses (SCCs) adopted by European Commission Decision 2021/914 or the relevant adequacy decisions are relied upon.

10.4. The selection of providers such as DeepSeek, whose servers may be in countries without an adequacy decision and where the achievement of an adequate level of protection through standard contracts must be separately assessed, is left to the Customer's assessment and responsibility. The Customer should assess the transfer conditions before using such providers for conversations containing personal data.

## 11. Support with data subject requests

11.1. The Customer can handle data subject requests itself using the tools offered by the Service (viewing, correcting, exporting and deleting a contact record; changing consent status). Contact deletion is permanent deletion.

11.2. If a data subject request reaches {{marka}} directly, {{marka}} does not respond to the request; to the extent permitted by law, it forwards the request to the Customer within 5 business days at the latest and informs the requester that the Customer is the data controller.

11.3. {{marka}} provides reasonable technical support to enable the Customer to fulfil its obligations under KVKK Art. 11 and Art. 13 within the 30-day statutory period. It also assists with matters such as data protection impact assessments and correspondence with the Authority, to the extent of the nature of the processing and the information available to {{marka}}.

## 12. Breach notification

12.1. When {{marka}} becomes aware of a Breach affecting Customer Personal Data, it notifies the Customer **without delay and in any event within 48 hours at the latest of becoming aware of it**, at the Owner e-mail address and, if any, the KEP address. This period has been set so that the Customer can notify the Authority within **72 hours** in accordance with Personal Data Protection Board Decision No. 2019/10.

12.2. The notification includes, to the extent known at the time: the nature and time of the Breach, the categories of data affected and the approximate number of persons, its likely consequences, the measures taken and proposed, and a contact person. Where not all information can be provided at the same time, it is provided in stages.

12.3. {{marka}} immediately takes the measures necessary to mitigate the effects of the Breach, supports the Customer in the notifications it will make to the Authority and data subjects, and keeps records of the Breach. The obligation to notify the Authority and data subjects rests with the Customer as data controller; {{marka}} does not make notifications on the Customer's behalf unless the Customer requests this in writing.

## 13. Audit

13.1. Upon the Customer's written request, {{marka}} provides within a reasonable time the information and documents demonstrating compliance with the DPA (a description of the security measures, the sub-processor list, the relevant policies). Audit requests are first met with these documents.

13.2. Where there are concrete and reasonable grounds that the documents are insufficient, where a Breach has occurred or where a competent authority so requests, the Customer may, with at least 30 days' prior written notice, no more than once a year, during business hours, and in a way that does not disrupt operations or endanger the confidentiality of other customers' data, carry out an audit itself or through an independent auditor bound by confidentiality obligations. The cost of the audit is borne by the Customer; if the audit reveals a material breach of the DPA by {{marka}}, the reasonable cost of the audit is borne by {{marka}}.

13.3. On-site audits cannot be carried out at the facilities of sub-processors (for example, AWS); for these sub-processors, the independent audit reports and compliance documents published by the providers are relied upon.

## 14. Return and deletion of data

14.1. For **30 days** after the Main Agreement ends for any reason, the Customer may export Customer Personal Data using the export tools offered by the Service (CSV, XLSX, PDF) or request support from {{eposta}}.

14.2. After this period expires, Customer Personal Data are deleted from live systems. Because encrypted backups are renewed on a cycle of at most **35 days**, the data are also permanently deleted from backups at the end of that period. While they remain in backups, the data are kept solely for disaster recovery purposes and are not processed for any other purpose.

14.3. The Customer may also delete contact and conversation data through the panel during the term of the contract. Upon the Customer's written request, {{marka}} confirms in writing that deletion has been completed.

14.4. Data that {{marka}} is required by law to retain (for example, invoice records) and data for which it is the data controller outside the scope of the DPA are excluded from this clause.

## 15. Liability

The parties' liability under the DPA is subject to the liability provisions of the Main Agreement. However, cases of intent or gross negligence and liability that cannot be limited by law are reserved. Each party is responsible for administrative fines and third-party claims arising from its breach of its own obligations; the Customer is responsible for consequences arising from processing in accordance with the Customer's instructions.

## 16. Term and precedence

16.1. The DPA remains in force for the term of the Main Agreement and for as long as {{marka}} continues to process Customer Personal Data.

16.2. In matters relating to the protection of personal data, in the event of a conflict between the DPA and the Main Agreement, the provisions of the DPA prevail; in the event of a conflict between the DPA and standard contracts signed between the parties, the provisions of the standard contracts prevail.

16.3. {{marka}} may update the DPA to comply with changes in the legislation. Material changes to the Customer's detriment are notified at least 30 days in advance.

16.4. Enterprise Plan Customers may contact {{kvkkEposta}} to request a signed copy of the DPA or a standard contract.
