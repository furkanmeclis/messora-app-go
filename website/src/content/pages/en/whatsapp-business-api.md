---
title: "What is the WhatsApp Business API?"
seoTitle: "What is the WhatsApp Business API? Full guide"
description: "What the WhatsApp Business API is, who it is for, what you can do with it and what it costs: Cloud API, requirements, messaging rules and common questions."
updated: 2026-11-28
faq:
  - q: "Is the WhatsApp Business API free?"
    a: "No. Meta charges for delivered messages by category and recipient country. Each number gets 1,000 free service messages a month. The platform you use has its own subscription on top."
  - q: "Do I need a developer to use the WhatsApp Business API?"
    a: "Not if you use a platform: the inbox, campaigns and automations run in the browser. A custom integration with your own systems may need technical help."
  - q: "How many people can I message?"
    a: "A new business portfolio can start conversations with 250 unique people per rolling 24 hours. Business verification and high-quality sending raise this step by step to 2,000, 10,000, 100,000 and unlimited. Replies to customers who wrote to you don't count."
  - q: "Can I keep my current WhatsApp number?"
    a: "Yes, but the account on the WhatsApp app has to be deleted; a number can't be on the app and the API at the same time. Meta's coexistence option allows both for some setups; check its current conditions."
  - q: "Is bulk messaging on the WhatsApp Business API legal?"
    a: "You can message people who opted in, using approved templates and giving them an easy way to opt out. In Turkey, Law No. 6563, the commercial electronic messages regulation, the İYS registry and KVKK also apply. Review your processes with a legal adviser."
---

The WhatsApp Business API is Meta's official interface for using WhatsApp through software instead of a phone app. It lets several agents talk to customers from one number, send campaigns and notifications with approved templates, connect WhatsApp to CRM and e-commerce systems, and run chatbots and AI agents.

This page brings together what it is, who it suits, how it works, what it costs and what you need to start.

## A short definition

WhatsApp has three faces for businesses:

- **WhatsApp**: the consumer app.
- **WhatsApp Business app**: a free phone app for small businesses, with a business profile, catalogue, quick replies and labels.
- **WhatsApp Business Platform (API)**: a software interface for larger-scale use. It has no screen of its own; you use it through a platform or your own software.

"Official WhatsApp API" refers to the third. Tools sold as a "WhatsApp API" that actually automate WhatsApp Web are not official and can get your number banned.

## Cloud API, On-Premises API and the app

| | Cloud API | On-Premises API | WhatsApp Business app |
|---|---|---|---|
| Runs on | Meta's servers | The business's own servers | A phone |
| Status | Current and recommended | Retired by Meta | Current |
| Setup | No servers needed | Required hosting and maintenance | Install the app |
| Best for | Teams, campaigns, integrations | No longer an option for new setups | Small businesses run by one person |

For a business starting today, the WhatsApp Business API in practice means the **Cloud API**. For a detailed comparison with the app, see [WhatsApp Business app vs API](/en/blog/whatsapp-business-app-vs-api).

## Who is it for?

It's about how you use WhatsApp more than company size. Consider the API if:

- several people answer the same number
- you get dozens or hundreds of messages a day and some go unanswered
- you want to send regular campaigns to opted-in customers and measure results
- you want order, shipping, appointment or payment updates sent automatically
- you want to connect WhatsApp to a CRM, e-commerce platform or your own software
- you want automatic replies or an AI first response after hours

| Industry | Typical use |
|---|---|
| [E-commerce](/en/solutions/ecommerce) | Order and shipping updates, cart reminders, returns |
| [Retail](/en/solutions/retail) | Campaigns, stock questions, loyalty |
| [Healthcare and clinics](/en/solutions/healthcare-and-clinics) | Appointment booking and reminders, intake questions |
| [Education](/en/solutions/education) | Enrolment, parent updates, class reminders |
| [Travel and hotels](/en/solutions/travel-and-hotels) | Booking confirmations, check-in details, guest requests |
| [Real estate](/en/solutions/real-estate) | Listing questions, lead capture, viewings |
| [Service businesses](/en/solutions/service-businesses) | Quotes, appointments, service follow-up |

## What you can do

- **Team inbox**: every agent works on the same number from the browser, with assignment, ownership and internal notes. History survives phone changes and staff turnover.
- **Campaigns**: send approved templates to opted-in customers, targeted by segment, tag or uploaded list, and measure delivery, reads, replies and opt-outs.
- **Notifications**: order confirmations, shipping updates, reminders, invoices and one-time passcodes sent from your systems.
- **Automations and chatbots**: welcome flows, opt-out handling, follow-ups for unanswered customers, built with triggers and conditions.
- **AI agents**: answer common questions, collect details and hand over to a person when unsure or when asked.
- **Integrations**: connect CRM, e-commerce and other systems through APIs and webhooks.

## The core rules

**The 24-hour customer service window.** When a customer writes, a 24-hour window opens in which you can send free-form messages. Outside it, or if they never wrote, you can only start a conversation with an **approved template**.

**Templates.** Pre-approved message formats in three categories: marketing, utility and authentication. Meta says review can take up to 24 hours.

**Opt-in.** Meta requires that people shared their number with you and agreed to receive messages from your business. In Turkey, commercial messaging law and the İYS registry apply too.

**Messaging limits.** A new portfolio can message 250 unique people outside the window per rolling 24 hours. Verification or high-quality sending raises this to 2,000, and then usage and quality take it to 10,000, 100,000 and unlimited. The limit applies per business portfolio, not per number.

**Quality rating.** Meta rates numbers and templates from customer feedback. Low quality stops limit increases, can pause templates and, in serious cases, restrict the account.

## Costs at a glance

**1. Meta's message fees.** Since 1 July 2025 Meta charges each delivered message by category and recipient country. Approximate rates for Turkish recipients since 1 April 2026:

| Category | Per message (USD) |
|---|---|
| Marketing | ≈ 0.0109 |
| Utility | ≈ 0.0009 |
| Authentication | ≈ 0.0009 |
| Service (replies) | 1,000 free per number per month; charged above that since 1 October 2026 |

Meta changes rates only on 1 January, 1 April, 1 July and 1 October, and TRY is not a billing currency. Check [Meta's pricing page](https://developers.facebook.com/docs/whatsapp/pricing) for current figures, and see our [WhatsApp pricing guide](/en/blog/whatsapp-pricing-2026-guide) for worked examples.

**2. Platform fees.** The subscription for the software you use the API through: inbox, campaigns, automations and AI.

## What you need to start

- **A Meta business portfolio** in Meta Business Suite
- **Business verification**: not required to start, but needed for higher limits, more numbers and more templates
- **A phone number** you own that can receive an SMS or voice call; if it's on the WhatsApp app, that account must be deleted
- **A display name** that clearly relates to your business
- **A payment method** on the WhatsApp account for Meta's fees
- **Approved templates** to start conversations

Step-by-step instructions: [How to apply for the WhatsApp Business API](/en/blog/what-is-whatsapp-business-api-how-to-apply).

## Ways to connect

| Route | How | For |
|---|---|---|
| Your own Meta app | App, system user and non-expiring token in Meta's developer dashboard | Teams that want full control |
| Embedded Signup | One-window setup on a provider's site with your Facebook login | Businesses that want a quick setup |
| Direct integration | Your software talks to Meta's API directly | Software teams building WhatsApp into their own product |

## Where Messora fits

Messora is a platform on the official WhatsApp Business Cloud API, built for small and mid-sized businesses in Turkey. On top of Meta's infrastructure it adds:

- [Team inbox](/en/features/team-inbox): assignment, ownership, internal notes, 24-hour window tracking
- [Campaigns](/en/features/campaigns): segments and lists, consent and opt-out management, Meta cost and a forecast before sending
- [Template builder](/en/features/template-builder): media headers, buttons, limited-time offers, carousels, OTP and Flows
- [Automations](/en/features/automations): a visual flow editor with ready-made presets
- [AI agents](/en/features/ai-agents): one AI profile per number, human handoff, lead capture
- [Analytics and reports](/en/features/analytics-and-reports): campaign results, forecast vs actual, exports
- [API and webhooks](/en/features/api-and-webhooks): scoped API keys and signed webhooks

You connect your own Meta app (App ID and System User token) today, with guided steps; one-click Embedded Signup is coming soon. Meta's fees are separate from the plan price and shown before you send. Data is hosted in AWS Frankfurt (EU); see [security](/en/security) for details.

Compare plans on the [pricing page](/en/pricing), start a 14-day trial without a card, or [book a demo](/en/demo).
