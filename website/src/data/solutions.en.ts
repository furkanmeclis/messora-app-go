import type { Solution } from './types';

export const solutions: Solution[] = [
  {
    key: "ecommerce",
    slug: "ecommerce",
    nav: "E-commerce",
    icon: "cart",
    seoTitle: "WhatsApp Business API for E-commerce",
    seoDescription: "Send order and shipping updates on WhatsApp, let AI answer the common questions, and send campaigns only to customers who opted in. Built for online stores.",
    eyebrow: "E-commerce",
    h1: "WhatsApp for e-commerce: automate order questions, leave selling to your team",
    lead: "Order confirmations, shipping details and return questions get answered on WhatsApp, and campaigns reach only customers who said yes. Messora connects to your store's system through the API and webhooks.",
    summary: "Order and shipping updates, AI answers to common questions and campaigns to customers who opted in.",
    pains: [
      {
        title: "\"Where is my order?\" swamps the team",
        body: "The same question is answered by hand all day. Meanwhile a customer asking about a product, ready to buy, waits in the queue and drifts away.",
      },
      {
        title: "No idea who a campaign reaches or what it costs",
        body: "Lists are mixed and nobody knows who wants messages. The cost only shows up on the invoice, and every message to someone who did not want it puts the number's quality at risk.",
      },
      {
        title: "One phone, several people",
        body: "The store's number lives on a single phone. Nobody knows who answered which customer; customers ask twice and some messages are never answered.",
      },
    ],
    flows: [
      {
        title: "Shipping update and tracking questions",
        trigger: "When an order ships",
        steps: [
          "Your store's system sends the shipping details to the Messora API",
          "A utility shipping template goes to the customer with the tracking number",
          "If the customer replies, the \"Automatic reply with AI\" flow answers from your delivery and returns rules",
          "For returns, damage or complaints, the AI hands the conversation to the team with a summary",
        ],
        outcome: "Routine shipping questions are answered before they reach the team, which handles the conversations that need a person.",
        features: ["api", "templates", "ai", "inbox"],
      },
      {
        title: "Cart reminder, only to customers who opted in",
        trigger: "When a customer leaves without completing checkout",
        steps: [
          "Your store's system adds a \"cart-reminder\" tag to the customer through the API",
          "An automation triggered by the tag waits a while, then sends the marketing template",
          "A customer without marketing consent gets nothing; the step is logged as \"skipped\"",
          "If the customer writes \"STOP\", consent is switched off at once and the \"Unsubscribe request\" flow thanks them",
        ],
        outcome: "Reminders go only to customers who want them, and your number's quality is protected.",
        features: ["api", "automations", "templates"],
      },
      {
        title: "New collection campaign",
        trigger: "When new products go live",
        steps: [
          "Create a segment by last purchase or interest",
          "Show the products card by card with a carousel marketing template",
          "Check blocked contacts, Meta cost and finish time in the pre-launch analysis",
          "The \"Positive customer response\" flow assigns interested customers to someone in sales",
        ],
        outcome: "You know the campaign's cost up front and connect interested customers with an agent straight away.",
        features: ["campaigns", "templates", "automations", "analytics"],
      },
    ],
    templates: [
      {
        name: "shipping_update",
        category: "utility",
        body: "Hello {{1}}, your order {{2}} has shipped. Your tracking number is {{3}}. You can send any delivery questions in this chat.",
        buttons: ["Track my parcel"],
      },
      {
        name: "order_confirmation",
        category: "utility",
        body: "Thank you {{1}}, we have received your order {{2}}. Total: {{3}}. We will let you know here when it ships.",
      },
      {
        name: "cart_reminder",
        category: "marketing",
        body: "Hello {{1}}, the items in your cart are still waiting for you. Use the link below to complete your order. If you would rather not receive these messages, just reply STOP.",
        buttons: ["Back to cart", "No more messages"],
      },
    ],
    compliance: "Marketing messages such as campaigns and cart reminders are commercial electronic messages under Turkish Law No. 6563: they need the customer's prior consent, and registering that consent in the Message Management System (İYS) is your responsibility. Send cart reminders only to customers with marketing consent. Messora keeps each customer's marketing and service consent separately and records the source and time of every change; a contact who opted out of marketing gets no marketing message through campaigns or automations. Order and shipping updates are utility templates. Meta prices marketing and utility messages differently, so choosing the category that fits the content matters for both compliance and cost.",
    faq: [
      {
        q: "Is there a ready integration for my store platform?",
        a: "There is no ready store plugin. Your store's system connects to Messora through the REST API and webhooks: a small integration that sends order and shipping events to the API drives notifications, tags and automations. API calls are available from the Growth plan.",
      },
      {
        q: "Can I show my product catalogue on WhatsApp?",
        a: "Meta's catalogue buttons are not supported, because Messora does not manage product catalogues. You can show products in a carousel template with up to 10 cards, each with an image and a link button.",
      },
      {
        q: "What does one cart reminder cost?",
        a: "A cart reminder is a marketing message. On the 1 April 2026 rate card, a delivered marketing message to Turkey costs about 0.0109 USD; check the current price on Meta's official page. For campaigns, Messora calculates the amount before sending.",
      },
      {
        q: "Who answers the questions that come in after a campaign?",
        a: "If you like, the AI agent gives the first answer and hands over when needed. The \"Campaign follow-up\" flow tags customers who reply to a campaign, leaves a note and routes them to an agent.",
      },
    ],
    features: ["campaigns", "automations", "ai", "api"],
  },
  {
    key: "retail",
    slug: "retail",
    nav: "Retail",
    icon: "store",
    seoTitle: "WhatsApp for Retail and Physical Stores",
    seoDescription: "WhatsApp for your stores: pickup notices, quick answers to store questions, limited-time coupon offers and city-based campaigns, all with consent checks.",
    eyebrow: "Retail",
    h1: "WhatsApp for retail: keep talking to store customers after the till",
    lead: "Tell customers when their reserved item arrives, answer opening-hours questions without a wait, and announce your sale with a coupon offer. All on the official WhatsApp Business API, with consent checks.",
    summary: "Ready-for-pickup notices, quick answers to store questions and coupon campaigns to customers who opted in.",
    pains: [
      {
        title: "Store questions keep the phone busy",
        body: "\"Do you have this in stock?\" and \"When do you close?\" come in all day. Staff at the till try to keep up with both customers and the phone.",
      },
      {
        title: "You have the number, not the consent",
        body: "Numbers collected at the till pile up in a list, but nobody recorded who agreed to campaign messages. Sending without consent is both a legal and a quality risk.",
      },
      {
        title: "No view of campaign results",
        body: "A sale announcement goes out, but nobody knows how many read it, who was interested or what it cost. The next campaign is planned on guesswork again.",
      },
    ],
    flows: [
      {
        title: "Reserved item is ready",
        trigger: "When a customer's reserved item reaches the store",
        steps: [
          "Your stock or sales system sends the details to the Messora API",
          "A utility \"item ready\" template goes out with the store name and pickup deadline",
          "If the customer replies, the conversation is assigned to that store's agent",
        ],
        outcome: "Customers hear about their item on time, and reserved items are not forgotten on the shelf.",
        features: ["api", "templates", "inbox"],
      },
      {
        title: "Store information questions",
        trigger: "When a customer asks about the address, opening hours or returns",
        steps: [
          "The \"Automatic reply with AI\" flow answers from the store details in your instructions",
          "For things the AI cannot know, such as stock, the conversation goes to the team",
          "The \"Handoff to an agent\" flow sends anyone who asks for a manager or live support straight to an agent",
        ],
        outcome: "Common questions are answered without a wait, and the team handles only the conversations that need it.",
        features: ["ai", "automations", "inbox"],
      },
      {
        title: "Sale announcement by city",
        trigger: "When a new sale starts",
        steps: [
          "Create a segment by the customer's city or store",
          "Build a limited-time offer template with a coupon code; WhatsApp shows the countdown",
          "The preflight counts only customers with marketing consent and shows the blocked ones with reasons",
          "When the offer ends the campaign stops by itself; review results broken down by city",
        ],
        outcome: "The announcement reaches only customers who agreed, and you see in figures which city responded.",
        features: ["campaigns", "templates", "analytics"],
      },
    ],
    templates: [
      {
        name: "item_ready",
        category: "utility",
        body: "Hello {{1}}, the {{2}} you reserved is waiting for you at our {{3}} store. We are holding it for you until {{4}}.",
        buttons: ["Get directions"],
      },
      {
        name: "seasonal_sale",
        category: "marketing",
        body: "Hello {{1}}, the seasonal sale has started at our {{2}} store. Use the coupon code below at the till for an extra discount. Reply STOP if you would rather not receive messages.",
        buttons: ["Copy code", "See the offer"],
      },
    ],
    compliance: "A phone number taken at the till or on a loyalty card form is not, on its own, consent to marketing messages. Sale and campaign announcements are commercial electronic messages: get the customer's explicit consent, record where and when it was given and register it in İYS. Messora keeps marketing and service consent separately and stores the source of every change; it does not send marketing templates to contacts without consent. Notices such as item ready or order status are utility templates. Meta prices the categories differently, and if you add promotional content to a utility template, Meta may classify it as marketing.",
    faq: [
      {
        q: "Can each of our stores have its own number?",
        a: "On Pro you connect 3 WhatsApp numbers and all their conversations come into the same inbox. For more numbers, the Enterprise plan is set to your needs.",
      },
      {
        q: "Can we send campaigns to the numbers we collected at the till?",
        a: "Only if the customer explicitly agreed to marketing messages. When you import numbers, marketing consent starts as \"unknown\"; you need to mark the customers who consented and register them in İYS.",
      },
      {
        q: "Do store teams talk only to their own customers?",
        a: "You can assign conversations to the relevant store's agent. By default the Agent role sees only conversations assigned to them and those not yet assigned; managers see all of them.",
      },
      {
        q: "What happens when a coupon offer expires?",
        a: "When the offer's end time arrives, the campaign linked to it pauses itself and the remaining customers wait. A campaign cannot be started with an expired offer.",
      },
    ],
    features: ["campaigns", "templates", "inbox", "ai"],
  },
  {
    key: "health",
    slug: "healthcare-and-clinics",
    nav: "Healthcare and clinics",
    icon: "health",
    seoTitle: "WhatsApp Appointment Reminders for Clinics",
    seoDescription: "WhatsApp for clinics: appointment reminders and confirmations, AI answers to after-hours questions, and clear rules that keep health data out of the chat.",
    eyebrow: "Healthcare and clinics",
    h1: "WhatsApp for clinics: remind patients of appointments, keep health data out of the chat",
    lead: "Appointment reminders and confirmations happen on WhatsApp, and a patient who wants to reschedule lands with the front desk. Messages stay limited to appointment logistics; diagnoses and test results stay out of this channel.",
    summary: "Appointment reminders and confirmations, a first answer to after-hours questions and rules that respect health data.",
    pains: [
      {
        title: "Confirming appointments by phone takes hours",
        body: "Tomorrow's patients are called one by one, and those who do not pick up are called again. A no-show is an empty appointment slot.",
      },
      {
        title: "After-hours questions pile up overnight",
        body: "Questions about the address, parking or how to book come in evenings and weekends. In the morning the team starts by working through the backlog.",
      },
      {
        title: "Patient details on personal phones",
        body: "Staff message patients from their own WhatsApp accounts. Nobody knows which details sit where, or what happens when someone leaves.",
      },
    ],
    flows: [
      {
        title: "Appointment reminder and confirmation",
        trigger: "One day before the appointment",
        steps: [
          "Your booking system sends the patient's name, date, time and branch to the Messora API",
          "A utility reminder template goes out with \"I'll be there\" and \"I want to reschedule\" buttons",
          "An automation triggered by \"I want to reschedule\" hands the conversation to the front desk with a note",
          "Delivery and read status of the reminder flows back to your booking system through webhooks",
        ],
        outcome: "The front desk deals only with patients who want a change, and calling everyone to confirm becomes less necessary.",
        features: ["api", "templates", "automations", "inbox"],
      },
      {
        title: "First answer after hours",
        trigger: "When a patient writes while the clinic is closed",
        steps: [
          "The AI agent answers logistics questions such as the address, opening hours and how booking works, following your instructions",
          "A medical question is treated as a critical topic and handed to the team",
          "For booking requests it asks for the name and preferred day; it does not ask for health information",
          "In the morning the team sees a summary of each conversation in the \"Waiting for a person\" list",
        ],
        outcome: "Patients know their message was seen, and the team starts the day with summaries and clear priorities.",
        features: ["ai", "inbox"],
      },
      {
        title: "First-time patient",
        trigger: "When a message arrives from a number that has never written before",
        steps: [
          "The \"Welcome new customer\" flow sends a short welcome and adds a \"new-customer\" tag",
          "The agent asks for the fields defined in the profile, such as name and preferred branch, one at a time",
          "Once the details are complete, the conversation goes to the booking team",
        ],
        outcome: "A new patient's basic contact details are collected without touching health data.",
        features: ["automations", "ai", "inbox"],
      },
    ],
    templates: [
      {
        name: "appointment_reminder",
        category: "utility",
        body: "Hello {{1}}, you have an appointment on {{2}} at {{3}} at our {{4}} branch. Please let us know below whether you can attend.",
        buttons: ["I'll be there", "I want to reschedule"],
      },
      {
        name: "appointment_booked",
        category: "utility",
        body: "Dear {{1}}, your appointment is booked: {{2}} at {{3}}. Address: {{4}}. Reply to this message if you need to change it.",
      },
      {
        name: "sign_in_code",
        category: "authentication",
        body: "{{1}} is your verification code. For your security, do not share this code.",
        buttons: ["Copy code"],
      },
    ],
    compliance: "Under Article 6 of Turkey's data protection law (KVKK), health data is a special category of personal data with stricter rules. Keep WhatsApp messages to appointment logistics (date, time, branch, address); do not put diagnoses, test results, medication or department details in templates or messages. Get the patient's explicit consent for appointment reminders and keep a record of it. Messora's AI profiles cannot be configured with fields such as health information or ID numbers, and the agent is told never to ask for or store them. If a patient volunteers health information, the message is kept in the conversation history, so let patients know not to share health details in this channel. Appointment notices are utility templates and sign-in codes are authentication templates; Meta prices the categories differently.",
    faq: [
      {
        q: "Can we send test results over WhatsApp?",
        a: "We advise against it. A test result is health data. Use the WhatsApp message only to say the result is ready and where to view it, and share the result itself in your own secure patient portal.",
      },
      {
        q: "What happens if a patient asks the AI a medical question?",
        a: "In the profile instructions you tell the agent not to give medical advice and to hand such questions to the team as a critical topic. You can try it with sample questions in the test console before going live.",
      },
      {
        q: "How do we record a patient's consent?",
        a: "Each contact's marketing and service consent is kept separately, and every change records the source, the time and who made it. You can record consent in the panel, through an import or through the API.",
      },
      {
        q: "Do doctors message patients from their own phones?",
        a: "No. The clinic's number is connected to Messora and the team writes from the panel. Conversations stay in the clinic's account; when someone leaves, you deactivate their user and assign their conversations to someone else.",
      },
    ],
    features: ["templates", "ai", "automations", "inbox"],
  },
  {
    key: "education",
    slug: "education",
    nav: "Education",
    icon: "education",
    seoTitle: "WhatsApp for Schools and Education",
    seoDescription: "WhatsApp for schools, courses and training centres: AI answers during enrolment, prospect follow-up, class reminders and consent-based messaging with parents.",
    eyebrow: "Education",
    h1: "WhatsApp for education: bring order to enrolment season and parent messages",
    lead: "Let the AI give the first answer to the flood of enrolment questions, collect a prospect's details as it talks, and leave your admissions team the serious candidates. Class and exam reminders go to parents as approved templates.",
    summary: "First answers during enrolment, prospect follow-up, class reminders and consent-based messaging with parents.",
    pains: [
      {
        title: "Enrolment season outpaces the team",
        body: "Questions about programmes, fees and dates pile up in the same few weeks. When an answer is slow, the prospect writes to another school.",
      },
      {
        title: "Prospects get lost in follow-up",
        body: "A prospect who asked for information is never called back, or three people call the same one. Who is at which stage lives in a spreadsheet that is out of date.",
      },
      {
        title: "Parent announcements are scattered",
        body: "Timetable changes, mock exams and parents' evenings are announced in different groups. Nobody knows who read them and who never got them.",
      },
    ],
    flows: [
      {
        title: "Enrolment questions",
        trigger: "When a prospective student or parent writes for the first time",
        steps: [
          "The \"Welcome new customer\" flow greets them and adds a \"new-customer\" tag",
          "The AI agent answers programme, calendar and fee questions from your instructions",
          "It asks for name, e-mail and the programme of interest during the conversation",
          "Once the details are complete, the conversation goes to admissions with a summary",
        ],
        outcome: "Admissions meets every prospect knowing what they want, with their details ready.",
        features: ["automations", "ai", "inbox"],
      },
      {
        title: "Class and exam reminder",
        trigger: "One day before a mock exam or class",
        steps: [
          "Put parents into groups by class",
          "Create a campaign for the group with a utility reminder template",
          "Follow delivery and read rates in the analysis centre",
        ],
        outcome: "You see which parents received the reminder and read it.",
        features: ["campaigns", "templates", "analytics"],
      },
      {
        title: "Following up quiet prospects",
        trigger: "When a prospect has not replied for 3 days after getting information",
        steps: [
          "The \"Detect customers who have not replied for a set time\" flow adds a \"silent\" tag",
          "For prospects with marketing consent, the \"Follow up unanswered customer\" flow sends an approved reminder template",
          "Prospects who reply are assigned to someone in admissions",
        ],
        outcome: "No prospect is forgotten, and follow-ups go only to those who agreed to receive messages.",
        features: ["automations", "templates", "inbox"],
      },
    ],
    templates: [
      {
        name: "class_reminder",
        category: "utility",
        body: "Dear {{1}}, this is a reminder about the {{2}} class on {{3}}. Time: {{4}}. We will let you know here if anything changes.",
      },
      {
        name: "enrolment_open",
        category: "marketing",
        body: "Hello {{1}}, enrolment for {{2}} is now open. Use the link below to browse our programmes and pre-register. Reply STOP if you would rather not receive messages.",
        buttons: ["Browse programmes", "Talk to an adviser"],
      },
    ],
    compliance: "If many of your students are under 18, communicate through parents and get consent from the parent. Keep children's data to a minimum: do not share grades, attendance or health information in messages. Messora's AI profiles cannot be configured with special fields such as ID numbers. Enrolment and promotional announcements are commercial electronic messages and need prior consent and İYS registration. Notices to enrolled students, such as class and exam reminders, are utility templates. Meta prices marketing and utility messages differently, so choose the template category by its content.",
    faq: [
      {
        q: "Can we send bulk announcements to parents?",
        a: "Yes. Put parents into groups and send a campaign with an approved template. Notices such as timetable changes are utility; enrolment and promotional announcements are marketing.",
      },
      {
        q: "What should we do if students write directly?",
        a: "You can answer the conversation, but do not send marketing messages to students under 18, and contact the parent about matters such as enrolment. Write this rule into the AI profile's instructions as well.",
      },
      {
        q: "How large can our admissions team be?",
        a: "Users: 2 on Starter, 5 on Growth, 12 on Pro, and set to your needs on Enterprise. You share the work by assigning conversations within the team.",
      },
      {
        q: "Where does the AI get fee and programme information?",
        a: "From the instructions you write in the agent's profile. Keep programmes, fees and dates up to date there; for anything it does not know, the agent hands the conversation to the team.",
      },
    ],
    features: ["ai", "automations", "campaigns", "inbox"],
  },
  {
    key: "travel",
    slug: "travel-and-hotels",
    nav: "Travel and hotels",
    icon: "plane",
    seoTitle: "WhatsApp Business API for Hotels and Travel",
    seoDescription: "WhatsApp for hotels, agencies and travel businesses: booking confirmations, pre-arrival details, AI answers to late-night questions and care with guest ID data.",
    eyebrow: "Travel and hotels",
    h1: "WhatsApp for travel and hotels: talk to guests from booking to checkout",
    lead: "Booking confirmations and arrival details go out on WhatsApp, the AI gives the first answer to late-night price questions, and the sales team starts the morning with the details ready. Guests' identity documents stay out of this channel.",
    summary: "Booking confirmations, pre-arrival details, first answers at night and feedback after the stay.",
    pains: [
      {
        title: "Questions arrive at every hour",
        body: "Guests ask about prices and availability from other time zones, in the middle of the night. A guest with no answer by morning books somewhere else.",
      },
      {
        title: "The same details asked again and again",
        body: "Check-in time, transfers, parking and breakfast hours come up with every booking. Reception types the same answers all day.",
      },
      {
        title: "ID photos in the chats",
        body: "Guests send passport and ID photos over WhatsApp. That data sits unmanaged on personal phones and in chat histories.",
      },
    ],
    flows: [
      {
        title: "Booking confirmation and arrival details",
        trigger: "When a booking is made",
        steps: [
          "Your booking system sends the details to the Messora API",
          "A utility confirmation template goes out with the booking number",
          "Before arrival, a second utility template shares transfer and check-in details",
          "If the guest taps \"I'd like a transfer\", the conversation is assigned to reception",
        ],
        outcome: "Guests get what they need before arrival, and reception handles only the requests.",
        features: ["api", "templates", "automations", "inbox"],
      },
      {
        title: "Late-night price and availability questions",
        trigger: "When a guest writes while the sales team is offline",
        steps: [
          "The AI agent answers questions about room types, facilities and cancellation terms from your instructions",
          "It asks for dates and the number of guests during the conversation",
          "Once the details are complete, the conversation goes to sales with a summary",
        ],
        outcome: "Guests who write at night are not left without an answer, and sales starts the morning with what it needs for a quote.",
        features: ["ai", "inbox"],
      },
      {
        title: "Feedback after the stay",
        trigger: "After the guest checks out",
        steps: [
          "Send a campaign with \"I was happy\" and \"I had a problem\" buttons to guests with marketing consent",
          "Map the buttons as positive and negative",
          "The \"Negative customer response\" flow leaves a note and hands guests who had a problem to the team",
          "The \"Positive customer response\" flow tags happy guests and assigns them to the right person",
        ],
        outcome: "You deal quickly with guests who had a problem and keep happy guests in a separate list.",
        features: ["campaigns", "automations", "analytics"],
      },
    ],
    templates: [
      {
        name: "booking_confirmation",
        category: "utility",
        body: "Dear {{1}}, your booking from {{2}} to {{3}} is confirmed. Your booking number is {{4}}. Check-in time: {{5}}.",
      },
      {
        name: "arrival_details",
        category: "utility",
        body: "Hello {{1}}, here are the details for your arrival on {{2}}: {{3}}. You can send any questions in this chat.",
        buttons: ["I'd like a transfer", "I have a question"],
      },
      {
        name: "stay_feedback",
        category: "marketing",
        body: "Hello {{1}}, thank you for staying with us at {{2}}. Would you share how it went? Reply STOP if you would rather not receive messages.",
        buttons: ["I was happy", "I had a problem"],
      },
    ],
    compliance: "Apply data minimisation to passport and ID details: do not ask guests for ID photos on WhatsApp; collect them at check-in or in your own secure system. Incoming media is kept in the conversation history. Messora's AI profiles cannot be configured with fields such as ID or passport numbers, and the agent is told not to ask for them. Booking confirmations and arrival details are utility templates; promotions and feedback requests are marketing templates, which need the guest's prior consent and, in Turkey, İYS registration. Meta charges differently by category and by the recipient's country, so messages to foreign guests are priced at that country's rate.",
    faq: [
      {
        q: "Can we message foreign guests in their own language?",
        a: "Yes. Each template is submitted for approval in one language, and you can create English or other language versions of the same template. You also set the language the AI agent replies in through the profile instructions.",
      },
      {
        q: "What happens if a guest sends a passport photo?",
        a: "Incoming media is kept in the conversation history. That is why we recommend asking for identity documents at check-in or in your own secure system rather than on WhatsApp. If needed, you can delete the contact and their message history.",
      },
      {
        q: "Can it connect to our booking system?",
        a: "There is no ready booking-system integration. Your system sends template messages through the REST API and receives incoming messages and delivery status through webhooks. API calls are available from the Growth plan.",
      },
      {
        q: "Does messaging numbers abroad cost more?",
        a: "It can. Meta sets the fee by the recipient's country and the template's category. Before a campaign, Messora calculates the cost by each recipient's country; countries missing from the rate card are not guessed but shown separately.",
      },
    ],
    features: ["templates", "ai", "api", "inbox"],
  },
  {
    key: "realestate",
    slug: "real-estate",
    nav: "Real estate",
    icon: "home",
    seoTitle: "WhatsApp Business API for Real Estate Agencies",
    seoDescription: "WhatsApp for estate agencies: AI greets listing enquiries, collects budget and area, viewing notices go out, and new listings reach consenting buyers.",
    eyebrow: "Real estate",
    h1: "WhatsApp for real estate: turn listing enquiries into qualified buyers",
    lead: "Let the AI give the first answer to listing enquiries, collect budget and area as it talks, and pass a ready prospect to the agent. Conversations stay in the agency's account, not on an agent's phone.",
    summary: "First answers to listing enquiries, prospect qualification, viewing notices and listing announcements to buyers who opted in.",
    pains: [
      {
        title: "Not every enquiry is a serious buyer",
        body: "Most messages from listing portals start and end with \"What's the price?\". Agents spend their days on first replies to find the real buyers.",
      },
      {
        title: "When an agent leaves, clients go too",
        body: "Chats live on agents' personal phones. When an agent leaves, the client history and open conversations leave with them.",
      },
      {
        title: "Bulk listing announcements are a legal risk",
        body: "Sending new listings to everyone whose number you have breaks commercial electronic message rules without consent. Complaints also lower the number's quality.",
      },
    ],
    flows: [
      {
        title: "From listing enquiry to qualified prospect",
        trigger: "When the first message about a listing arrives",
        steps: [
          "The AI agent answers general questions about the listing from your instructions",
          "It asks for budget, area and number of rooms one at a time during the conversation",
          "Once the details are complete, the conversation goes to an agent with a summary and the collected details",
          "The agent checks the collected details and verifies them with one click",
        ],
        outcome: "The agent starts every conversation knowing what the prospect is looking for.",
        features: ["ai", "inbox"],
      },
      {
        title: "Viewing appointment",
        trigger: "When a viewing is scheduled",
        steps: [
          "A utility appointment template goes out with the address, time and agent's name",
          "If the prospect taps \"I want to change the time\", the conversation is assigned to the agent",
          "The \"Handoff to an agent\" flow passes anyone asking for a manager straight to the team",
        ],
        outcome: "Viewing details arrive in one message, and change requests reach the right agent.",
        features: ["templates", "automations", "inbox"],
      },
      {
        title: "New listing announcement",
        trigger: "When a new property is added to the portfolio",
        steps: [
          "Create a segment by area and budget",
          "Show the listings in a carousel marketing template",
          "The preflight counts only prospects with marketing consent",
          "The \"Positive customer response\" flow assigns interested prospects to an agent",
        ],
        outcome: "The announcement reaches only prospects who agreed and match the criteria.",
        features: ["campaigns", "templates", "automations"],
      },
    ],
    templates: [
      {
        name: "viewing_appointment",
        category: "utility",
        body: "Hello {{1}}, your viewing on {{2}} at {{3}} at {{4}} is booked. Your agent: {{5}}.",
        buttons: ["I want to change the time"],
      },
      {
        name: "new_listings",
        category: "marketing",
        body: "Hello {{1}}, we have new listings in {{2}} that match what you are looking for. See the details below. Reply STOP if you would rather not receive messages.",
        buttons: ["View listings", "Talk to an agent"],
      },
    ],
    compliance: "In Turkey, new listing and portfolio announcements are commercial electronic messages under Law No. 6563 and the Regulation on Commercial Communication and Commercial Electronic Messages. Having written to you through a listing portal is not consent to marketing messages: get the prospect's explicit consent before sending announcements, record its source and register it in İYS. Messora sends no marketing templates to prospects who opted out or have not consented, and switches off consent at once for anyone who writes \"STOP\". A viewing appointment notice is a utility template; Meta prices marketing and utility messages differently.",
    faq: [
      {
        q: "What happens to conversations when an agent leaves?",
        a: "Conversations stay in the agency's account. You deactivate the agent's user and assign their conversations to another agent. The assignment history and audit log are kept.",
      },
      {
        q: "Can we send listings to people who contacted us through a portal?",
        a: "You can reply within 24 hours to the conversation about the listing they asked about. Announcing other listings is a marketing message and needs the prospect's explicit consent.",
      },
      {
        q: "Can we see which agent is talking to which prospect?",
        a: "Yes. Every conversation has an owner, and its assignment and handoff history is shown on the conversation. Managers see all conversations; agents see the ones assigned to them by default.",
      },
      {
        q: "Will the AI negotiate prices?",
        a: "The agent follows the rules you write in its profile. If you define price and negotiation as critical topics, those questions go straight to an agent.",
      },
    ],
    features: ["ai", "inbox", "campaigns", "automations"],
  },
  {
    key: "services",
    slug: "service-businesses",
    nav: "Service businesses",
    icon: "wrench",
    seoTitle: "WhatsApp Appointments for Service Businesses",
    seoDescription: "WhatsApp for salons, garages, repair and cleaning businesses: appointment reminders, job-done notices, alerts for unanswered messages and quick price answers.",
    eyebrow: "Service businesses",
    h1: "WhatsApp for service businesses: appointments, updates and price questions in one place",
    lead: "Appointment reminders go out on their own, customers hear when the job is done, and messages are not left without an owner while the team is out on a job. For any business that works by appointment, from salons to garages.",
    summary: "Appointment reminders, job-done notices, alerts for unanswered messages and a first answer to price questions.",
    pains: [
      {
        title: "Appointments get forgotten",
        body: "Customers forget their appointment and the empty slot is lost. Nobody has time to call or message each one as a reminder.",
      },
      {
        title: "The team is out, messages wait",
        body: "A technician in the middle of a job cannot check the phone. A customer asking for a price hears nothing for hours and calls someone else.",
      },
      {
        title: "\"Is my car ready?\" calls",
        body: "Customers phone to ask how the job is going. Every call is an interruption to the team's work.",
      },
    ],
    flows: [
      {
        title: "Appointment reminder",
        trigger: "One day before the appointment",
        steps: [
          "Your booking system passes the details through the API, or you use a group of tomorrow's customers",
          "A utility reminder template goes out with \"Confirm\" and \"I want to reschedule\" buttons",
          "An automation triggered by \"I want to reschedule\" assigns the conversation to an agent",
        ],
        outcome: "Customers remember their appointment, and those who want a change reach the team directly.",
        features: ["templates", "api", "automations"],
      },
      {
        title: "Job-done notice",
        trigger: "When the service or job is finished",
        steps: [
          "Your job tracking system sends the status to the Messora API",
          "A utility template shares the total and the pickup time",
          "The customer's questions land in the inbox and are assigned to the person who did the job",
        ],
        outcome: "Customers do not call to ask, and the team works without interruptions.",
        features: ["api", "templates", "inbox"],
      },
      {
        title: "Alert for unanswered messages",
        trigger: "When a customer's message has gone unanswered for 30 minutes",
        steps: [
          "Set up an automation with the \"no reply for a set time\" trigger",
          "Add an internal note saying the customer has been waiting 30 minutes",
          "Hand the conversation to the team's shared pool; people in the team are notified",
        ],
        outcome: "No customer message is left without an owner.",
        features: ["automations", "inbox"],
      },
      {
        title: "First answer to price questions",
        trigger: "When a customer asks about a service or price",
        steps: [
          "The AI agent explains the general price range and process from your service list",
          "It asks for what is needed for an exact quote: type of service, address, a suitable day",
          "Once the details are complete, it hands the conversation to the team",
        ],
        outcome: "Customers get a first answer without waiting, and the team follows up with what it needs for a quote.",
        features: ["ai", "inbox"],
      },
    ],
    templates: [
      {
        name: "appointment_reminder",
        category: "utility",
        body: "Hello {{1}}, you have an appointment tomorrow at {{2}} for {{3}}. If you cannot make it, please let us know below.",
        buttons: ["Confirm", "I want to reschedule"],
      },
      {
        name: "job_done",
        category: "utility",
        body: "Hello {{1}}, your {{2}} is complete. Total: {{3}}. You can collect it until {{4}}.",
      },
    ],
    compliance: "Appointment reminders and job-done notices are utility templates as long as they contain no promotion. If you add a discount or a new service announcement, Meta may classify the template as marketing, and marketing messages need the customer's prior consent and, in Turkey, İYS registration. Meta prices the categories differently. You can reply with free-form text within 24 hours of the customer's last message; since 1 October 2026, service messages beyond 1,000 free per number per month are also charged. Check the current rules on Meta's official pricing page.",
    faq: [
      {
        q: "We have no booking system. Can we still use it?",
        a: "Yes. Add tomorrow's customers to a group and send a campaign with the reminder template, or send template messages one by one from the inbox. Variables such as date and time are filled from contact fields.",
      },
      {
        q: "How does a team out on jobs see messages?",
        a: "The team sees and answers messages in the panel, and new messages arrive without a refresh. If you assign the conversation to the person doing the job, the customer talks to them directly.",
      },
      {
        q: "What does one reminder message cost?",
        a: "On the 1 April 2026 rate card, a delivered utility message to Turkey costs about 0.0009 USD. Check the current price on Meta's official page.",
      },
      {
        q: "How many of our staff can use the same number?",
        a: "2 users on Starter, 5 on Growth, 12 on Pro, and set to your needs on Enterprise. All of them answer the same number's conversations from the panel.",
      },
    ],
    features: ["templates", "automations", "inbox", "ai"],
  },
];
