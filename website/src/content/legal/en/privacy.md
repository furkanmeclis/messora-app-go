---
title: "Privacy Policy"
description: "How personal data are protected on the Messora website and WhatsApp Business API service: our roles, the data we collect, security measures and your rights."
updated: 2026-11-28
version: "1.0"
---

This Privacy Policy explains in general terms how personal data are processed and protected when you use the {{web}} website operated by {{unvan}} ("**{{marka}}**", "**we**") and the {{marka}} software service built on the official WhatsApp Business Cloud API (the "**Service**").

This English version is provided for convenience; the Turkish version prevails in case of any discrepancy.

Detailed information for data subjects in Türkiye is provided in the [Privacy Notice (KVKK)](/en/legal/privacy-notice). For visitors located in the European Union or the European Economic Area, explanations relating to the General Data Protection Regulation (EU) 2016/679 ("**GDPR**") are given in section 9, to the extent applicable.

## 1. Scope

This Policy covers the following activities:

- visiting our website and filling in our forms;
- registering for the Service, account management, subscription, payment and invoicing;
- support and sales correspondence with {{marka}}.

This Policy does not cover **customer data** that our Customers upload to the Service or process through the Service (for example, contact lists, WhatsApp messages, media files, campaign and automation data). For such data, the allocation of roles described in section 2 below applies.

## 2. Our roles: data controller and data processor

- **{{marka}} is the data controller** for the account, contact, contract, payment and security data of website visitors, form submitters, Customer representatives and Users.
- **{{marka}} is the data processor** for customer data that the Customer uploads to the panel or processes through the Service. For such data, the data controller is the relevant Customer company. The Customer decides which persons will be sent messages, which data will be uploaded, how long they will be retained and how data subjects will be informed. We process such data only on the Customer's instructions and for the purpose of providing the Service. This relationship is governed by the [Data Processing Agreement](/en/legal/data-processing-agreement).

If you have received a WhatsApp message from a business through {{marka}} and have a request concerning your data, please contact the business that sent the message first. If your request reaches us, we will forward it to the relevant business and support it in responding.

WhatsApp messages are delivered by Meta Platforms Ireland Ltd. (WhatsApp) under the WhatsApp Business terms that the Customer has accepted with Meta. How Meta processes such data is subject to Meta's own terms and policies.

## 3. Data we collect

- **Visit data:** IP address, browser and device information, time of visit, request logs. Usage data collected through analytics and marketing cookies are processed only if you give consent.
- **Form and contact data:** first name, last name, e-mail, telephone, company, job title, message content.
- **Account data:** first name, last name, e-mail, role, password hash, two-factor authentication data, session and login records.
- **Commercial data:** plan, order, invoice details (legal name, tax office and number, address; for sole proprietorships, Turkish national identity number), payment records.
- **Usage and security data:** features used, limit consumption, AI credit usage, error logs, audit log.

Data categories, purposes and legal grounds are shown in detail for each data subject group in the [Privacy Notice (KVKK)](/en/legal/privacy-notice).

## 4. Purposes for which we use data

- To provide the website and the Service, and to open and manage your account;
- to ensure authentication, authorisation and security;
- to respond to your requests and support correspondence;
- to carry out pricing and invoicing and fulfil legal obligations;
- to send mandatory notifications relating to the Service (e-mail verification, password reset, security alerts, invoice and plan notifications);
- to prevent misuse, spam and unauthorised access;
- if you give consent, to measure website usage and evaluate advertising conversions;
- if you give consent, to send you product news and announcements.

We do not sell your personal data. We do not use customer data for our own marketing activities and we do not use them to train artificial intelligence models.

## 5. Artificial intelligence features

The artificial intelligence features in the Service (AI agents that answer customer questions, information collection, inbox suggestions) run **only when the Customer enables them**. The Customer selects the provider (OpenAI, Anthropic, Google Gemini or DeepSeek) itself. When these features are used, the relevant conversation content is sent only to the selected provider.

- Requests to OpenAI are sent with the `store: false` option; by default, OpenAI does not use API data for model training.
- Under its commercial terms, Anthropic does not use API data for model training.
- Google does not use Gemini API requests from projects with billing enabled (paid projects) to improve its models; a key belonging to a paid project must therefore be used.
- DeepSeek's servers may be located in the People's Republic of China. Before using this provider for conversations containing personal data, the Customer must assess the provider's terms and the conditions for international transfers.

Meta prohibits the use of data obtained from the WhatsApp platform for model training; {{marka}} complies with this rule.

## 6. Data security

The main technical and organisational measures we take are:

- The Service is hosted in the Frankfurt (Germany, eu-central-1) region of Amazon Web Services.
- Data separation between Customers is ensured by PostgreSQL row-level security; every record is tagged with the company to which it belongs.
- Passwords are stored one-way hashed with Argon2id; the password itself is not stored.
- Two-factor authentication (TOTP and single-use recovery codes) is supported.
- Role-based authorisation (Owner, Admin, Manager, Agent) is applied.
- Secrets such as Meta access tokens, API keys and AI provider keys are stored encrypted and only their last characters are shown in the interface; API keys are kept as one-way hashes.
- Critical operations are written to an immutable audit log.
- Data are backed up in encrypted form every night.
- Connections are encrypted with HTTPS; rate limiting and account lockout are applied to login attempts.
- Message contents and telephone numbers are not written to application logs.
- Outgoing webhook requests are signed with HMAC-SHA256; requests targeting the server's internal network (SSRF) are blocked.

No system provides absolute security. If we become aware of a personal data breach, we make the necessary notifications in accordance with the KVKK and the decisions of the Personal Data Protection Board (Kurul). For breaches affecting customer data, we inform the Customer within the period specified in the [Data Processing Agreement](/en/legal/data-processing-agreement).

## 7. Sharing and international transfers

We share personal data only with service providers necessary to provide the Service (hosting, e-mail delivery, payment, invoice integration), with competent public authorities where there is a legal obligation, and with our legal advisers. The current list of sub-processors is on the [Sub-processors](/en/legal/subprocessors) page.

International transfers are made under Article 9 of the KVKK, as a rule using the standard contracts announced by the Personal Data Protection Board. For transfers subject to the GDPR, the European Commission's standard contractual clauses (SCCs) or the relevant adequacy decisions are relied upon.

## 8. Retention periods

We retain data for as long as the purpose of processing requires and the legislation provides. The main periods are: contract and invoice records 10 years; form records not converted into a contract 2 years; website traffic logs at least 1 and at most 2 years. After the contract ends, Customer data can be exported for 30 days and are then deleted; they may remain in encrypted backups for up to a further 35 days. All periods are set out in the [Privacy Notice (KVKK)](/en/legal/privacy-notice).

## 9. Additional information for visitors in the European Union (GDPR)

Where the GDPR applies:

- **The data controller** is {{unvan}}; you can contact us at {{kvkkEposta}}.
- **Legal bases:** performance of a contract (Art. 6(1)(b)), legal obligation (Art. 6(1)(c)), legitimate interest (Art. 6(1)(f); security, prevention of misuse, development of the service) and consent (Art. 6(1)(a); analytics and marketing cookies, commercial messages).
- **Your rights:** the rights of access, rectification, erasure, restriction of processing, data portability, objection to processing based on legitimate interest, and to withdraw your consent at any time. You may lodge a complaint with the data protection supervisory authority in your country of residence.
- **Transfers:** for transfers to Türkiye and to third countries, the European Commission's standard contractual clauses or adequacy decisions are used.
- **Automated decision-making:** we do not take decisions based solely on automated processing that produce legal effects concerning you or similarly significantly affect you.

## 10. Your rights and requests

You can find your rights under Article 11 of the KVKK and the methods of submitting a request in the [Privacy Notice (KVKK)](/en/legal/privacy-notice). You may submit your request through the form at {{basvuruUrl}}, to the KEP (registered electronic mail) address {{kep}}, or from your e-mail address registered in our system to {{kvkkEposta}}.

## 11. Children's privacy

Our website and the Service are intended for businesses; they are not intended for persons under 18. We do not knowingly collect personal data from persons under 18. If we become aware that such data have been provided to us, we take the necessary steps to delete them. In sectors where their own customers may include children, our Customers are responsible for taking the additional measures required by the relevant legislation.

## 12. Cookies

The cookies used on our website, their purposes and durations are explained in the [Cookie Policy](/en/legal/cookie-policy). Non-essential cookies run only if you give consent.

## 13. Changes

We may update this Policy in line with changes in legislation or in our service. The current version is published on this page with its effective date. We also announce significant changes affecting our Customers by e-mail or panel notification.

## 14. Contact

{{unvan}} · {{adres}} · KEP: {{kep}} · E-mail: {{kvkkEposta}} · Telephone: {{telefon}}
