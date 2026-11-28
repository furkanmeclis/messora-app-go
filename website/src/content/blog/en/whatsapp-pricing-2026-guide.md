---
title: "WhatsApp Business API pricing in 2026: a practical guide"
seoTitle: "WhatsApp API pricing guide 2026"
description: "WhatsApp API pricing in 2026: the per-message model, categories, Turkey rates, the October 2026 changes, billing currency and worked examples to cut costs."
date: 2026-11-18
updated: 2026-11-28
translationKey: "pricing"
tags: ["WhatsApp API pricing", "Meta fees", "Costs"]
faq:
  - q: "How much does a WhatsApp Business API message cost in Turkey?"
    a: "Since 1 April 2026 Meta has charged about USD 0.0109 per delivered marketing message and about USD 0.0009 per utility or authentication message to Turkish numbers. Rates can change at the start of each quarter, so check Meta's official pricing page."
  - q: "Are replies to customers free?"
    a: "Since 1 October 2026 each business phone number gets 1,000 free service messages a month. Service messages beyond that are charged, and utility templates sent inside an open window are charged too."
  - q: "Can I pay Meta in Turkish lira?"
    a: "No. TRY is not one of the currencies Meta bills WhatsApp in. Turkish businesses usually open a USD account and pay by card; the account currency can't easily be changed later."
  - q: "Do I pay for messages that aren't delivered?"
    a: "No. Under per-message pricing Meta charges only for delivered messages."
---

"How much will each message cost?" is the most common question about the WhatsApp Business API. The answer depends on the message category, the recipient's country and how many messages you send. This guide covers Meta's 2026 pricing model, the Turkey rates, the changes that took effect on 1 October 2026, and worked examples.

Two costs are worth separating first:

- **Meta's fees**: what Meta charges for each delivered message, whichever platform you use.
- **Platform fees**: the subscription for the software that gives you an inbox, campaigns and automations. This guide covers Meta's fees only.

> Figures in this guide were compiled on 28 November 2026. Meta can update prices four times a year. Always check [Meta's official pricing page](https://developers.facebook.com/docs/whatsapp/pricing) before making decisions.

## The model: per-message pricing

Meta moved from conversation-based to **per-message pricing** on 1 July 2025:

- Only **delivered** messages are charged.
- The rate depends on the message **category** and the recipient's **market**, i.e. the country code of their number.
- Each message has one category and is charged at most once.

A marketing message to a +90 number is billed at Turkey's marketing rate; one to a +49 number at Germany's. Where your business is located does not matter.

## Categories

| Category | Used for | When you can send it |
|---|---|---|
| Marketing | Campaigns, offers, new products, cart reminders, re-engagement | Any time, with an approved template |
| Utility | Order confirmations, shipping updates, appointment reminders, invoices | Any time, with an approved template |
| Authentication | One-time passcodes, login verification | Any time, with an approved template |
| Service | Free-form replies to customers | Only within 24 hours of the customer's last message |

You choose a template's category, but Meta has the final say. A utility template that contains promotional content is approved as marketing, or moved to marketing later.

## Turkey rates

Turkey is a standalone market on Meta's rate card. The rates in effect since 1 April 2026, which also appear on Turkish providers' pricing pages:

| Category | Per delivered message (USD) |
|---|---|
| Marketing | ≈ 0.0109 |
| Utility | ≈ 0.0009 |
| Authentication | ≈ 0.0009 |
| Service | 1,000 free per number per month; beyond that, at the utility rate |

Meta's page says Turkey's utility and authentication rates were lowered on 1 April 2026 and that Turkey was not among the markets whose rates changed on 1 October 2026. The exact figures are in Meta's downloadable rate card.

Utility and authentication also have **volume tiers**: as your monthly volume of charged messages in a market grows, messages in higher tiers are billed at lower rates. Tiers count across all WhatsApp accounts in a business portfolio and reset monthly.

Meta can change rates only on the first day of a quarter (**1 January, 1 April, 1 July, 1 October**), with at least one month's notice for rate card updates.

## What changed on 1 October 2026

**Service messages are no longer unlimited and free.** Each business phone number gets 1,000 free service messages per month. Unused messages don't roll over. Service messages beyond the free tier are charged; without a payment method, they aren't delivered.

**Utility templates inside an open window are charged again.** From July 2025 until October 2026, utility templates sent within 24 hours of the customer's message were free. Since 1 October 2026 they are billed at the normal utility rate.

In practice this means a busy support team, or a chatbot that answers every message, can now create Meta costs too. In Turkey the amount per message is small, but it is worth tracking as volume grows.

Some providers describe the free quota as per WhatsApp account rather than per number; Meta's own page says "each business phone number".

### What stays free

- The first 1,000 service messages per number per month
- Messages in the **free entry point** window opened by Click-to-WhatsApp ads or a Facebook Page call-to-action (see Meta's page for conditions)
- Messages that are not delivered

## Billing currency: no lira

Meta bills WhatsApp in 16 currencies: USD, EUR, GBP, AUD, IDR, INR, AED, ARS, BRL, CLP, COP, MXN, MYR, PEN, SAR and SGD. **TRY is not on the list.** Turkish businesses usually open a USD account and pay by card. The account currency can't be changed freely after creation, so choose deliberately, and budget with an exchange-rate assumption you update regularly. Ask your accountant about VAT treatment of services bought from abroad.

## Worked examples

The examples below assume **1 USD = 50.5 TRY**. That is an assumption; use the current rate and recalculate. Figures exclude VAT and bank charges and ignore volume-tier discounts.

| Category | USD | TRY at 50.5 (approx.) |
|---|---|---|
| Marketing | 0.0109 | 0.55 |
| Utility / authentication / service above quota | 0.0009 | 0.045 |

| Scenario | Volume | USD | TRY (assumed rate) |
|---|---|---|---|
| Seasonal campaign (marketing) | 5,000 | 54.50 | ≈ 2,752 |
| Order and shipping updates (utility) | 3,000 | 2.70 | ≈ 136 |
| Support replies above the free 1,000 (service) | 1,500 | 1.35 | ≈ 68 |
| Login codes (authentication) | 10,000 | 9.00 | ≈ 455 |
| **Total** | | **67.55** | **≈ 3,411** |

Two things stand out. First, **marketing makes up most of the bill**, so that is where savings are. Second, category matters: the same 3,000 order updates sent as marketing would cost about USD 32.70 (roughly 1,650 TRY at the assumed rate) instead of USD 2.70.

## How to reduce costs

1. **Categorise correctly.** Orders, shipping, appointments and invoices are utility, about a twelfth of the marketing rate in Turkey. Keep promotions out of utility templates; Meta moves mixed templates to marketing and applies escalating enforcement to businesses that repeatedly disguise marketing as utility.
2. **Segment.** Sending everything to everyone is the most expensive approach and hurts your quality rating. Target recent buyers, interests or locations.
3. **Use the service window well.** Answer the customer's question fully in the first reply. Since October 2026, inside an open window a free-form service message is often the better choice than a utility template.
4. **Make it easy for customers to write first.** Put a WhatsApp link or QR code on your site, invoices and shop counter. Click-to-WhatsApp ads open a free entry point window.
5. **Clean undeliverable numbers.** They aren't charged, but they distort reports and waste sending capacity.
6. **Honour opt-outs instantly.** Marketing to someone who opted out is a legal risk and wasted money.
7. **Watch frequency.** Meta may withhold some marketing messages to a person who receives too many, and read rates drop.

## Questions to ask a provider

Meta's fee is the same for everyone, but how it reaches you differs:

- Who pays Meta: you, by card, or the provider from a prepaid balance?
- Is there a markup on Meta's fee, per category?
- Which exchange rate and date are used for local-currency balances?
- Are free service messages ever charged by the provider?
- Are undelivered messages refunded?
- Can you see the cost before sending a campaign?

## How Messora shows the cost before you send

Before launching a campaign in Messora you see a **pre-launch analysis**: how many recipients are eligible and how many are blocked by consent or opt-out, the expected duration and finish time, the **estimated Meta cost** from Meta's rate card per country and category, and expected performance when there are at least 3 past campaigns and 500 recipients (otherwise it says there isn't enough history). After sending, each message is priced from the category Meta reports and the rate valid that day. See [campaigns](/en/features/campaigns) and [analytics](/en/features/analytics-and-reports).

Messora's subscription plans are separate from Meta's fees; see the [pricing page](/en/pricing).
