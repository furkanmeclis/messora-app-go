---
title: "What is the WhatsApp Business API and how do you apply?"
seoTitle: "How to apply for the WhatsApp Business API"
description: "How to get the WhatsApp Business API: Meta business portfolio, business verification, phone number, display name, messaging limits and common rejection reasons."
date: 2026-11-12
updated: 2026-11-28
translationKey: "api-apply"
tags: ["WhatsApp Business API", "Onboarding", "Meta"]
faq:
  - q: "Do I need a provider to get the WhatsApp Business API?"
    a: "No. Meta's Cloud API is open to any business. You can create your own Meta app and connect with a System User token. A provider's Embedded Signup flow only shortens the setup."
  - q: "Can I use the number I already have on the WhatsApp app?"
    a: "Yes, but under Meta's rules a number registered on WhatsApp cannot be registered on the Cloud API until the app account is deleted. Meta's coexistence option allows both for some setups; check its current conditions."
  - q: "Is business verification mandatory?"
    a: "Not to start. A new, unverified portfolio can start conversations with 250 unique people in a rolling 24 hours. Higher limits, more phone numbers and more templates need verification or are much easier with it."
  - q: "How does the messaging limit grow?"
    a: "Verification, or 2,000 high-quality delivered messages within 30 days, raises it to 2,000. After that Meta moves you up one level automatically when quality is high and you used at least half of your limit in the last 7 days."
---

The WhatsApp Business API is Meta's official interface for using WhatsApp through software rather than a phone. This guide gives a short recap of what it is and then focuses on the practical part: what you need, how to apply step by step, and where applications usually get stuck. Where local detail matters, we use Turkey as the example.

For the full overview, see our guide [What is the WhatsApp Business API?](/en/what-is-whatsapp-business-api)

## The short version

Today "WhatsApp Business API" in practice means Meta's **Cloud API**. Messages travel through Meta's servers, so you don't run any infrastructure. The older On-Premises API, which businesses hosted themselves, has been retired by Meta.

With the API you can:

- let several agents work on one number from a shared inbox
- send campaigns, order updates and one-time passcodes with approved templates
- connect WhatsApp to your CRM, e-commerce stack and internal systems
- run automations, chatbots and AI agents

In return you follow a few rules: if the customer hasn't written to you in the last 24 hours you can only start a conversation with a Meta-approved template, you need the recipient's opt-in, and delivered messages are charged by category.

## What to prepare before you apply

### 1. A Meta business portfolio

You need a **business portfolio** (formerly Business Manager) in Meta Business Suite. Your WhatsApp Business Account (WABA), phone numbers, Meta app and user permissions all live under it.

- Add at least two admins. A portfolio tied to one person becomes a problem when that person leaves.
- Make the legal name, address and website match your official documents exactly. Verification looks for that consistency.
- Use an e-mail address on your own domain.

### 2. Business verification

Business verification is Meta checking that the company in the portfolio is real and matches what you entered. It is not required to start, but according to Meta's developer documentation it is one of the ways to:

- raise the messaging limit from 250 to 2,000
- raise the number of registered phone numbers per portfolio from two to 20
- raise the template limit per WhatsApp account from 250 to 6,000 (together with at least one number that has an approved display name)

For a Turkish company, documents such as the tax certificate (vergi levhası), trade registry gazette or a utility bill in the company's name usually work. The accepted document list can change, so follow what the verification screen asks for.

### 3. A phone number

Meta's requirements for a Cloud API number:

- You own it, and it has a country and area code. Short codes are not supported.
- It can receive an SMS or a voice call for the verification code.
- **A number already in use on WhatsApp cannot be registered unless that account is deleted first.** Once registered, it keeps working for normal calls and SMS but can no longer be used in the WhatsApp or WhatsApp Business app.
- You set a two-step verification PIN during registration. Keep it safe; you need it to change or remove the number later.

If you want to move a number that is on the WhatsApp Business app today, you either delete the app account (chat history does not come across) or use Meta's **coexistence** option, which keeps the number working in both the app and the API and can sync up to six months of one-to-one history. Coexistence is set up through Embedded Signup by a Tech Provider or Solution Partner and has limitations; check the current conditions on [Meta's coexistence page](https://developers.facebook.com/documentation/business-messaging/whatsapp/embedded-signup/onboarding-business-app-users). For a full comparison see [WhatsApp Business app vs API](/en/blog/whatsapp-business-app-vs-api).

### 4. A display name

The display name is what customers see on your number's profile. Meta reviews it, and according to its documentation the name is re-verified automatically when the number reaches a higher messaging limit. The core of Meta's [display name guidelines](https://www.facebook.com/business/help/757569725593362):

- It must clearly relate to your business, matching your website, signage or invoices.
- It can't be only a generic word ("Furniture", "Hairdresser").
- Use natural capitalisation; all caps is only for real acronyms.
- No emoji, excess punctuation or anything that suggests another brand.

You can change it later, but every change is reviewed again and the number must be re-registered after approval.

## Step by step: Cloud API with your own Meta app

Menu names change from time to time, but the flow is:

1. **Open a Meta developer account** at developers.facebook.com with a login that has access to your portfolio.
2. **Create an app** of type "Business" and connect it to your portfolio.
3. **Add the WhatsApp product.** Meta gives you a test number and a temporary token for a first try.
4. **Add your real number** in WhatsApp Manager, enter the display name, verify by SMS or voice call and set the PIN.
5. **Add a payment method** to the WhatsApp account. The account currency can't be changed freely later, and **TRY is not one of Meta's billing currencies**; most Turkish businesses choose USD.
6. **Create a System User** in Business Settings and assign your app and WhatsApp account to it.
7. **Generate a permanent token** with the `whatsapp_business_messaging` and `whatsapp_business_management` permissions and expiry set to "Never". The temporary token from the app dashboard stops working within hours.
8. **Configure the webhook**: a public HTTPS callback URL, a verify token and subscriptions to the fields you need, starting with `messages`.
9. **Submit your first templates.** Meta says a decision can take up to 24 hours; most come back sooner.
10. **Test end to end**: send a template to your own phone, reply, answer inside the 24-hour window and confirm the webhook receives each event.

## The alternative: Embedded Signup through a provider

Meta's **Embedded Signup** turns the setup into one window on a provider's site: you log in with Facebook, choose or create the portfolio, verify the number and the provider receives the permissions it needs. No app, system user or token copying. Only Meta-approved Tech Providers and Solution Partners can offer it.

In Messora, one-click Embedded Signup is **coming soon**. Today you connect your own Meta app (App ID and System User token), and the panel walks you through it.

## Messaging limits

The messaging limit is the number of unique people you can message **outside the customer service window** in a rolling 24 hours. Replies to customers who wrote to you don't count. According to Meta's current documentation the limit applies to the **business portfolio**, shared by all its numbers.

| Level | Unique recipients per 24 h |
|---|---|
| New portfolio | 250 |
| Level 1 | 2,000 |
| Level 2 | 10,000 |
| Level 3 | 100,000 |
| Top | Unlimited |

To move from 250 to 2,000, do one of the following: verify your business, have the partner who onboarded you verify it, or send 2,000 delivered messages outside the window to unique numbers within a rolling 30 days using high-quality templates. Meta then reviews quality and raises the limit if approved.

Beyond 2,000, Meta raises the limit by one level within six hours when your quality is high across all numbers and templates and you used at least half of your current limit in the last seven days. Spreading campaigns across the week grows the limit more reliably than one large blast.

## Quality rating

Meta rates each number and template green, yellow or red based on customer feedback such as blocks, reports and low read rates. Low quality stops limit increases. A template that hits red is paused for 3 hours the first time and 6 hours the second; the third time it is disabled. Repeated policy violations can restrict the account.

The way to keep quality high is predictable: message only people who opted in, keep content relevant, don't overdo frequency, make opting out easy and never send marketing to someone who opted out.

## Common reasons applications get stuck

| Problem | Usual cause | Fix |
|---|---|---|
| Business verification rejected | Name or address differs from the document; unreadable or outdated document | Match portfolio details to the document exactly and upload a clear, current one |
| Display name rejected | Can't be linked to the business, or too generic | Use the brand name that appears on your website and documents |
| Number can't be registered | Still registered on the WhatsApp or WhatsApp Business app | Delete the app account or consider coexistence |
| No verification code | Line can't receive SMS or sits behind a switchboard | Request a voice call; check call forwarding |
| Sending stops after a few hours | Temporary token in use | Generate a non-expiring System User token |
| Templates can't be sent | No payment method, or template not approved yet | Check payment method and template status in WhatsApp Manager |

A banned number, often one previously used with unofficial bulk-messaging tools, has to go through Meta's appeal process before it can be registered.

## Checklist

- Business portfolio created with at least two admins
- Legal name, address and website match official documents
- Documents ready for business verification
- Number chosen; not on the WhatsApp app, or a migration plan in place
- Number can receive SMS or a voice call
- Display name follows Meta's guidelines
- Two-step verification PIN stored safely
- Payment method added, account currency chosen deliberately
- Non-expiring System User token in use
- Webhook configured and subscribed
- First templates submitted
- Opt-in collection and opt-out path ready

## Setting up with Messora

Messora runs on the official WhatsApp Business Cloud API. You connect your own Meta app with an App ID and a System User token; Messora checks the token with Meta, warns you before it expires and imports your numbers and templates. One-click Embedded Signup is coming soon.

Once connected, the [team inbox](/en/features/team-inbox), [template builder](/en/features/template-builder) and [campaigns](/en/features/campaigns) are ready in the same panel. See the plans on the [pricing page](/en/pricing), or [book a demo](/en/demo) to go through the setup together.
