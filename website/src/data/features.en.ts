import type { Feature } from './types';

export const features: Feature[] = [
  {
    key: "inbox",
    slug: "team-inbox",
    nav: "Team inbox",
    icon: "inbox",
    seoTitle: "WhatsApp Team Inbox with Conversation Assignment",
    seoDescription: "Run WhatsApp conversations as a team: assignment, internal notes, a customer panel with consent status and the 24-hour rule enforced, on the official API.",
    eyebrow: "Team inbox",
    h1: "WhatsApp team inbox: every conversation has an owner",
    lead: "Stop losing customer messages on one shared phone. Each conversation is assigned to someone, the team talks in internal notes, and the customer's history and consent status sit in the side panel. New messages arrive without a page refresh.",
    summary: "Share conversations across the team, talk in internal notes and see each customer's history and consent on one screen.",
    highlights: ["Assignment and ownership", "Updates in real time", "24-hour rule enforced"],
    blocks: [
      {
        title: "Every conversation has an owner",
        body: "Take a conversation yourself or assign it to a teammate. Unassigned conversations wait in a shared pool. By default the Agent role sees only conversations assigned to them and those not yet assigned; owners, admins and managers see everything.",
        points: [
          "Assignment and status history is kept on the conversation",
          "A closed conversation reopens when the customer writes again",
          "Assignment changes are written to the audit log",
        ],
      },
      {
        title: "Internal notes the customer never sees",
        body: "Leave a note for a teammate inside the conversation. Internal notes are never sent to the customer. Notes marked \"for the AI\" become instructions for the AI agent, which does not pass them on to the customer.",
      },
      {
        title: "Customer panel",
        body: "Next to the conversation you see the customer's tags, notes, groups and their consent status, kept separately for marketing and service messages. Details the AI collected during the chat are marked \"not verified\"; check them and verify with one click.",
        points: [
          "Marketing and service consent kept apart",
          "Source and time of every consent change recorded",
          "Switch the AI off for a single contact",
        ],
      },
      {
        title: "Text, media and templates",
        body: "Write text or attach an image, video, audio file or document; file type and size limits are checked before sending. If the customer's 24-hour window has closed, the inbox points you to an approved template.",
      },
      {
        title: "The rules apply themselves",
        body: "Under Meta's rules, free-form text can only be sent within 24 hours of the customer's last message, and the inbox shows that window on the conversation. A customer who writes \"STOP\" has their marketing consent switched off at once and the event goes into the conversation history. When a customer says in their own words that they want no more messages, the person responsible for the conversation is notified.",
      },
      {
        title: "Working alongside the AI",
        body: "In suggestion mode the AI drafts a reply to each customer message; you accept, edit or reject it. When the AI hands a conversation over, it lands in the \"Waiting for a person\" list with a summary. The moment an agent replies, they take the conversation over.",
      },
    ],
    steps: [
      { title: "Connect your number", body: "Connect your WhatsApp Business account and number from Meta. Incoming messages land straight in the inbox." },
      { title: "Invite your team", body: "Give each person the Owner, Admin, Manager or Agent role, and turn on two-factor authentication if you like." },
      { title: "Share out conversations", body: "Take or assign incoming conversations, and use automations to route certain ones straight to the right person." },
      { title: "Follow up and close", body: "Close the conversation when it is done. If the customer writes again, it reopens with its full history." },
    ],
    plans: "The inbox is in every plan. Users: 2 on Starter, 5 on Growth, 12 on Pro, and set to your needs on Enterprise. On Pro you handle the conversations of 3 WhatsApp numbers in the same inbox.",
    faq: [
      {
        q: "Can several people use the same WhatsApp number?",
        a: "Yes. The number is connected to Messora, and as many people as your plan's user limit answer its conversations from the panel. Nobody has to share a phone.",
      },
      {
        q: "Can agents see each other's conversations?",
        a: "Not by default. An agent sees conversations assigned to them and those not yet assigned. Seeing all conversations is a permission of the Owner, Admin and Manager roles.",
      },
      {
        q: "How do I write to a customer after 24 hours?",
        a: "With a template approved by Meta. Free-form text cannot be sent outside the window; the inbox checks this and takes you to template selection. When the customer replies, a new 24-hour window opens.",
      },
      {
        q: "What happens if a customer writes \"STOP\"?",
        a: "Their marketing consent is switched off at once and the event is recorded in the conversation history. An automatic confirmation can be sent if you turn it on. They are skipped in later campaigns automatically, and you can still answer them in the conversation.",
      },
      {
        q: "Does the inbox work with our own CRM?",
        a: "Yes. New message, assignment and closing events can be pushed to your system with signed webhooks, and the API lets you read conversations and reply. API calls are available from the Growth plan.",
      },
    ],
    related: ["ai", "automations", "templates"],
  },
  {
    key: "campaigns",
    slug: "campaigns",
    nav: "Campaigns",
    icon: "megaphone",
    seoTitle: "WhatsApp Bulk Messaging and Campaigns",
    seoDescription: "Send WhatsApp bulk messages on the official API: consent checks, Meta cost up front, duration forecast, pause and resume, and auto-stop on quality warnings.",
    eyebrow: "WhatsApp campaigns",
    h1: "WhatsApp bulk messaging: see who it reaches and what it costs before you send",
    lead: "Pick your audience by segment, group, tag or an uploaded list. Before anything is sent, Messora shows how many people can receive the message, who is blocked and why, what Meta will charge and when sending will finish.",
    summary: "Send template messages to customers who opted in, and see cost, duration and blocked contacts up front.",
    highlights: ["Meta cost before sending", "Consent checked on every send", "Stops itself on quality warnings"],
    blocks: [
      {
        title: "Build the audience your way",
        body: "Send a campaign to a segment, a group, a tag, all contacts or a list you upload as CSV or Excel. Nobody is added to the audience twice.",
        points: [
          "CSV and XLSX import with column mapping, progress and an error report",
          "Dynamic segments keep the audience current by rules",
          "Scheduled sending in the time zone you choose",
        ],
      },
      {
        title: "Know who cannot be reached beforehand",
        body: "The preflight counts your audience against the category of the template you chose: who can receive it, who gets a warning and who is blocked, with the reasons. Contacts who opted out of marketing, stopped your messages in WhatsApp or whose number cannot be reached get nothing, and no setting can override that.",
        points: [
          "Blocked contacts counted by reason",
          "A \"warn\" or \"block\" policy for contacts with unknown consent",
          "Consent checked once more right before each message goes out",
        ],
      },
      {
        title: "See the cost before you send",
        body: "Meta's fee is calculated from Meta's rate card for each recipient's country and the template's category. A country missing from the rate card is never guessed; it is shown on its own line as \"not in rate card\". Meta's message fees are separate from your plan.",
      },
      {
        title: "Know when it will finish, without made-up forecasts",
        body: "The pre-launch analysis works out duration and finish time from your number's sending capacity and the other campaigns running on it, and says how many 24-hour periods sending will take if Meta's messaging limit is not enough. Expected read and reply rates appear only when you have at least 3 completed campaigns and 500 recipients in the same category; otherwise it says plainly \"not enough history\".",
      },
      {
        title: "One number, several campaigns, a fair queue",
        body: "Campaigns sending from the same number share its capacity fairly by priority, and your inbox replies are not held up by campaign load. You can change a running campaign's priority and rate cap.",
        points: [
          "Pause: unsent messages are held",
          "Before resuming, see who it will go to and how long it will take",
          "Cancel: the remaining messages are never sent",
        ],
      },
      {
        title: "Automatic stop that protects your number",
        body: "A campaign pauses itself when Meta reports a spam rate-limit error, your account is restricted, number or template quality drops to red, Meta pauses the template, or opt-outs pass a threshold. You see which rule tripped and at what value, and resume once you have checked.",
      },
    ],
    steps: [
      { title: "Choose a template", body: "Pick a template approved by Meta and map its variables to contact fields." },
      { title: "Choose the audience", body: "Select a segment, group, tag or uploaded list; the preflight counts it by consent." },
      { title: "Read the pre-launch analysis", body: "Check eligible and blocked contacts, Meta cost, duration and finish time." },
      { title: "Launch and watch", body: "Start now or schedule it. Delivery, reads and replies update live." },
    ],
    plans: "Campaigns and consent management are in every plan. Contacts are unlimited on all plans, with fair use on Starter. Starter and Growth send from 1 WhatsApp number, Pro from 3. Meta's per-message fees are not included in the plan price and are shown before sending.",
    faq: [
      {
        q: "Is sending bulk messages on WhatsApp allowed?",
        a: "Sending through the official WhatsApp Business API, with a template Meta approved, to people who opted in follows Meta's rules. In Turkey, marketing messages are commercial electronic messages and need the recipient's prior consent and registration in İYS. Messora keeps consent records; registering in İYS is your responsibility.",
      },
      {
        q: "How much do I pay per message?",
        a: "Meta charges for delivered template messages by country and category. On the 1 April 2026 rate card, a marketing message to Turkey is about 0.0109 USD and a utility or authentication message about 0.0009 USD. Check the current price on Meta's official pricing page; Messora calculates the amount before you start a campaign.",
      },
      {
        q: "Can I stop a campaign halfway?",
        a: "Yes. When you pause, unsent messages are held. Before resuming you see who it will go to, with consent checked again, and the estimated time. If you cancel, the remaining messages are never sent.",
      },
      {
        q: "What happens if my number's quality drops?",
        a: "With campaign protection on, which is the default, a campaign pauses itself when number or template quality turns red. You can change the thresholds under campaign protection in Settings.",
      },
      {
        q: "Can I upload my Excel list and send to it?",
        a: "Yes. Upload a CSV or XLSX file and map the columns; rows with errors are reported. New contacts start with marketing consent \"unknown\". Do not send marketing to lists without consent; you can record consent during import or update it later.",
      },
      {
        q: "Can you tell me the read rate in advance?",
        a: "Only when you have enough history: at least 3 completed campaigns and 500 recipients in the same category. Without that, the analysis does not invent a rate; it says \"not enough history\" and shows what is missing.",
      },
    ],
    related: ["templates", "analytics", "automations"],
  },
  {
    key: "ai",
    slug: "ai-agents",
    nav: "AI agents",
    icon: "sparkle",
    seoTitle: "WhatsApp AI Agent and Chatbot",
    seoDescription: "A WhatsApp AI agent that answers your customers, collects their details and hands over to your team when unsure. Works with OpenAI, Anthropic and Google Gemini.",
    eyebrow: "AI agents",
    h1: "WhatsApp AI agent: it answers, and hands over to your team when it is unsure",
    lead: "An agent running on OpenAI, Anthropic or Google Gemini models answers your customers following your business's instructions. When a customer asks for a person, or the agent is not sure of the answer, it hands the conversation to your team with a summary.",
    summary: "AI agents that answer customer questions, collect details and hand over to the team when needed.",
    highlights: ["OpenAI, Anthropic and Gemini", "Hands over when asked", "Chats not used for training"],
    blocks: [
      {
        title: "Three ways to work",
        body: "Each WhatsApp number has one AI profile: instructions, model choice and handoff rules. Run the agent the way you need.",
        points: [
          "Auto reply: the agent writes to the customer directly",
          "Approval: the agent writes, and it is sent once an agent approves",
          "Suggestions only: the agent drafts, you send the reply",
        ],
      },
      {
        title: "A path to a person, always",
        body: "When a customer asks for an agent, live support or a real person, the handoff cannot be switched off; Meta's policy requires a quick, clear path to a human. The agent also hands over when its confidence is low, it has no answer, it detects a critical topic, an action is outside what it may do, or the 24-hour window has closed. The team receives a summary, the customer's need, a suggested next step and the details collected.",
      },
      {
        title: "Collects customer details as it talks",
        body: "The agent asks for name, e-mail, city or other fields you define one or two at a time, after answering, inside the conversation; it never makes the customer fill in a form. Once the details are complete it can hand the conversation to your sales team.",
        points: [
          "Saved values are marked \"collected by AI, not verified\"",
          "Never overwrites a value a person entered or verified",
          "Health data, ID and card numbers cannot be configured as fields",
        ],
      },
      {
        title: "Takes instructions from your team",
        body: "Internal notes you add to a conversation as \"for the AI\" become instructions in the agent's next replies; the agent does not pass them on to the customer. You decide in the profile which actions the agent may take: creating contacts, adding tags, assigning, starting automations.",
      },
      {
        title: "Try before going live, and see the cost",
        body: "In the test console, run the saved profile through a sample conversation and see the reply, its confidence, the tokens used and the cost; nothing is sent to a customer. Every AI request is recorded in the usage log. When the hourly reply limit or the monthly budget is reached, the agent hands the conversation to the team.",
      },
      {
        title: "Your conversations are not used for training",
        body: "Conversations go only to the provider you choose. OpenAI requests ask for the response not to be stored (store: false); Anthropic and Google do not train on data from paid API projects. The first AI message starts with a disclosure telling the customer they are talking to an AI.",
      },
    ],
    steps: [
      { title: "Turn the AI on", body: "Switch AI on in Settings and choose the default provider and model." },
      { title: "Write the profile", body: "For your number, enter instructions describing your business, your rules and when to hand over." },
      { title: "Test it in the console", body: "Try the profile with sample questions and see the replies and the cost." },
      { title: "Go live", body: "Start in suggestion mode and move to auto reply once you trust the results." },
    ],
    plans: "1 AI agent on Starter, 3 on Growth, 10 on Pro. Monthly AI credits: 300 on Starter, 2,000 on Growth, 8,000 on Pro. Enterprise has a custom credit pool and the option to use your own AI keys.",
    faq: [
      {
        q: "Which AI models can I use?",
        a: "OpenAI, Anthropic (Claude) and Google Gemini models are supported, and DeepSeek can be connected too. You choose a default model for the company and can switch to another model on any number's profile.",
      },
      {
        q: "What if the AI tells a customer something wrong?",
        a: "To reduce the risk, use approval mode: the agent writes the reply and it is not sent until an agent approves it. Replies below the confidence threshold and critical topics go to the team. You can test the profile in the console before going live.",
      },
      {
        q: "What happens when a customer wants to talk to a person?",
        a: "The agent hands the conversation to the team straight away. This rule cannot be switched off. The conversation goes into the \"Waiting for a person\" list, the right people are notified, and whoever takes it sees the summary.",
      },
      {
        q: "Are our conversations used to train AI models?",
        a: "No. Meta forbids training models on WhatsApp data. Conversations go only to the provider you choose; OpenAI requests use store: false, and Anthropic and Google do not train on paid API data.",
      },
      {
        q: "How are AI credits used?",
        a: "Replies, suggestions, handoff summaries, classifying campaign replies and AI steps in automations all use credits. Every request is recorded in the usage log, and the panel shows usage by purpose and by campaign.",
      },
      {
        q: "Does the agent reply after the 24-hour window closes?",
        a: "No. Free-form text cannot be sent while the window is closed, so an agent on auto reply hands the conversation to the team. Reaching the customer again needs an approved template.",
      },
    ],
    related: ["inbox", "automations", "analytics"],
  },
  {
    key: "automations",
    slug: "automations",
    nav: "Automations",
    icon: "flow",
    seoTitle: "WhatsApp Automated Messages and Flows",
    seoDescription: "Build WhatsApp automated message flows in a visual editor: 11 ready presets for welcome messages, handoff to an agent, chasing silent customers and follow-ups.",
    eyebrow: "Automations",
    h1: "WhatsApp automated messages: hand repetitive work to flows",
    lead: "Welcome first-time customers, pass anyone who asks for an agent to the team, follow up with those who went quiet. You build flows on a visual canvas from a trigger and steps, and the path each run took is shown on the canvas.",
    summary: "Build flows from triggers and steps in a visual editor, and start in minutes from ready presets.",
    highlights: ["Visual flow editor", "11 ready presets", "Loop and duplicate protection"],
    blocks: [
      {
        title: "Build on a visual canvas",
        body: "A flow starts with a trigger and continues with steps. Condition steps split into yes and no paths and branch up to four levels deep. Undo, redo, automatic layout and a mini map keep large flows readable.",
      },
      {
        title: "When it starts",
        body: "Triggers are tied to real WhatsApp events.",
        points: [
          "A message arrives (by keyword or any message), a first-time contact writes",
          "A tag is added, a conversation is closed, a message is delivered, read or fails",
          "A subscription request, the 24-hour window about to close, no reply for a set time",
          "A first reply to a campaign, a positive or negative response, an AI handoff",
        ],
      },
      {
        title: "What it does",
        body: "Reply, send a template, add or remove a tag, add to a group, update a field, wait (up to 30 days), assign, close the conversation, mute, add an internal note, make an HTTP request and send an e-mail. AI steps detect intent, extract details, analyse sentiment, place the contact in a segment, draft a message or hand the conversation to the AI agent or to a person.",
      },
      {
        title: "Ready presets",
        body: "Ready flows for common needs are in every plan. The copy you install is yours to edit as you like. It cannot be switched on until its missing settings, such as which template to send, are filled in.",
        points: [
          "Welcome new customer, Unsubscribe request, Handoff to an agent",
          "Collect customer details (AI), Automatic reply with AI",
          "24-hour window reminder, Follow up unanswered customer",
          "Campaign follow-up, Positive customer response, Negative customer response",
          "Detect customers who have not replied for a set time",
        ],
      },
      {
        title: "No loops, no broken rules",
        body: "Each event starts a flow at most once, and changes made by one automation do not trigger another. You can set a cooldown per contact. Even if a message step is retried, the message is not sent twice. Consent and 24-hour rules apply here too: a step that cannot apply is logged as \"skipped\" and the flow carries on.",
      },
      {
        title: "A record of every run",
        body: "Every run and every step is logged, and the path a run took is highlighted on the canvas. When a step fails, the reason is shown. HTTP, e-mail, message and AI steps can retry: on temporary errors the step waits and runs again.",
      },
    ],
    steps: [
      { title: "Pick a starting point", body: "Install one of the ready presets or start with an empty canvas." },
      { title: "Set the trigger", body: "Choose which event, on which number and under which condition the flow starts." },
      { title: "Add the steps", body: "Drag message, tag, condition, wait and handoff steps onto the canvas and connect them." },
      { title: "Switch it on and watch", body: "Activate the flow and follow its run history and step log on the canvas." },
    ],
    plans: "3 active flows on Starter, 20 on Growth, unlimited on Pro and Enterprise. Ready presets are in every plan. AI steps use your plan's monthly AI credits.",
    faq: [
      {
        q: "Do I need to code to build automations?",
        a: "No. You build flows by connecting nodes on the visual canvas. Starting from a preset and changing only the texts and tags is often enough.",
      },
      {
        q: "Can an automation send a customer a message they did not want?",
        a: "No. Automations follow the same consent rules as campaigns: a contact who opted out of marketing gets no marketing template, and no free-form text is sent while the 24-hour window is closed. A step that cannot apply is skipped and logged.",
      },
      {
        q: "Does the 24-hour window reminder message the customer?",
        a: "No. This preset leaves a note for your team before the window closes and routes the conversation to an agent. Sending filler messages to keep the window open breaks Meta's rules, and no preset does it.",
      },
      {
        q: "Can automations connect to our other systems?",
        a: "Yes. The HTTP request step calls HTTPS addresses, and requests to private network addresses are blocked. The e-mail step sends a notice to the address you choose. You can also push events to your system with signed webhooks.",
      },
      {
        q: "How do I find out why a flow did not run?",
        a: "The run history shows the result of every step, completed, skipped or failed, with the reason. The path the run took is highlighted on the canvas, and AI nodes carry a warning badge while AI is switched off.",
      },
    ],
    related: ["ai", "inbox", "campaigns"],
  },
  {
    key: "templates",
    slug: "template-builder",
    nav: "Template builder",
    icon: "template",
    seoTitle: "WhatsApp Message Template Builder",
    seoDescription: "Build WhatsApp message templates in the panel: media headers, buttons, variables, carousels, limited-time offers and one-time codes. Track Meta approval here.",
    eyebrow: "Template builder",
    h1: "WhatsApp message templates: a builder that checks the rules as you type",
    lead: "Build templates with text or media headers, buttons, variables, carousels, limited-time offers and one-time verification codes in the panel. Messora checks the limits Meta documents as you type, submits the template for approval and tracks its status.",
    summary: "A template builder that checks Meta's limits as you type; submit for approval and follow the status here.",
    highlights: ["Limits checked as you type", "Approval status syncs itself", "Carousels and limited-time offers"],
    blocks: [
      {
        title: "Header, body, footer and buttons",
        body: "The header can be text, an image, a video, a document or a location. The body runs to 1,024 characters and carries variables such as {{1}} and {{2}}, each with the sample value Meta asks for. A template can have up to 10 buttons.",
        points: [
          "Template buttons: quick-reply button, website link, phone, copy code and voice call",
          "The sample file for a media header is uploaded to Meta from the builder",
          "Character and button limits shown as you type",
        ],
      },
      {
        title: "Carousel",
        body: "Show 2 to 10 swipeable cards in marketing templates. Each card has an image or video, an optional short text and up to two buttons; all cards share the same layout.",
      },
      {
        title: "Limited-time offer",
        body: "Build the limited-time offer template, which WhatsApp shows with a countdown, with a copy-coupon button and a link button. You define the offer once with its title, discount, coupon and end date, and the campaign fills them into the template. When the offer ends, the campaign linked to it pauses itself.",
      },
      {
        title: "One-time codes (OTP)",
        body: "Create templates in the authentication category for sign-in and verification. Meta sets the message text; you choose a copy code, one-tap or zero-tap autofill button and a code validity of 1 to 90 minutes. You send the code through the API.",
      },
      {
        title: "WhatsApp Flows",
        body: "Attach a WhatsApp Flow you built at Meta to a template as a button, and the customer opens the form or booking screen inside WhatsApp. Flow screens are designed on Meta's side.",
      },
      {
        title: "Approval and status tracking",
        body: "Submit the template to Meta from the builder. Status changes such as approval, rejection or pausing, and the quality rating, reach Messora on their own, and the templates already in your WhatsApp Business account are synced. If Meta pauses a template, the campaign using it stops too.",
      },
    ],
    steps: [
      { title: "Pick category and language", body: "Choose marketing, utility or authentication, and the template's language." },
      { title: "Write the content", body: "Write the header, body and buttons and give sample values for variables; limits are checked instantly." },
      { title: "Submit for approval", body: "Send the template to Meta; its approval status updates in the panel on its own." },
      { title: "Use it everywhere", body: "Use the approved template in campaigns, the inbox, automations and the API." },
    ],
    plans: "The template builder and limited-time offers are in every plan. Approval, quality ratings and the number of templates per account are set by Meta.",
    faq: [
      {
        q: "Why do I need a message template?",
        a: "Meta requires every business-initiated message, and every message sent more than 24 hours after the customer's last one, to be an approved template. Inside the window you can write free-form text; campaigns, reminders and notifications need templates.",
      },
      {
        q: "Which category should I choose?",
        a: "Promotions, discounts and product announcements are marketing; order, appointment and account notices are utility; one-time codes are authentication. Meta prices the categories differently and may change a category that does not fit the content.",
      },
      {
        q: "What do I do if my template is rejected?",
        a: "The rejection shows in the panel. Fix the content to fit Meta's rules, create the template again and resubmit it. Sample values for variables and the category choice are among the most common reasons for rejection.",
      },
      {
        q: "How long does approval take?",
        a: "Meta decides. When the status changes, Meta notifies Messora and the panel updates by itself; you do not need to refresh or check at Meta separately.",
      },
      {
        q: "How do I fill variables for each customer?",
        a: "In a campaign you map each variable to the contact's name, phone, e-mail, extra fields or custom attributes. When sending through the API, you pass the values in the request.",
      },
    ],
    related: ["campaigns", "automations", "api"],
  },
  {
    key: "analytics",
    slug: "analytics-and-reports",
    nav: "Analytics and reports",
    icon: "chart",
    seoTitle: "WhatsApp Campaign Analytics and Reports",
    seoDescription: "See delivery, reads, replies, positive and negative responses, opt-outs and cost for every WhatsApp campaign. Compare forecast with actuals, export reports.",
    eyebrow: "Analytics and reports",
    h1: "WhatsApp campaign reports: who read it, who replied, what it cost",
    lead: "See every campaign's delivery, reads, replies, positive and negative responses, opt-outs and cost on one screen. Each figure carries its source: reported by Meta, or calculated by the system.",
    summary: "See campaign results with sourced figures, break them down, compare them with the forecast and download the report.",
    highlights: ["Every figure has a source", "Forecast and actuals side by side", "Excel, CSV and PDF reports"],
    blocks: [
      {
        title: "Campaign analysis centre",
        body: "Audience, blocked, sent, delivered, read, failed, replied, positive, negative and opted out; rates and cost. Figures update live while the campaign runs. Delivery and reads come from Meta; replies, reactions and cost are marked \"System calculation\".",
      },
      {
        title: "Replies credited to the right campaign",
        body: "A customer's reply or opt-out counts for the campaign that reached them last, but only if it arrives on the number the campaign sent from and within the window you set (7 days by default, 1 to 30 days). Automations use the same rule, so reports and flows always point to the same campaign.",
      },
      {
        title: "Positive and negative responses",
        body: "Map your template's quick reply buttons to positive, negative or neutral, and the label is set when the customer taps. Free-text replies can be classified by the AI if you want. A label set by hand is never changed by the system.",
      },
      {
        title: "Breakdowns",
        body: "See the same figures split by group, segment, tag, contact type or the value of a custom attribute. Compare at a glance which audience replied more or opted out more.",
      },
      {
        title: "Forecast and actuals",
        body: "The pre-launch analysis is fixed when the campaign starts. As it runs, actual figures appear next to the forecast with the difference. The forecast includes rates only when there is enough history; without it, no rate is invented.",
      },
      {
        title: "Recipients table and exports",
        body: "Filter recipients by status, tag, group, segment or custom attribute; tag the ones you select, add them to a group or create a follow-up campaign draft for them. Download the filtered list as CSV or Excel.",
        points: [
          "Excel report with Summary, Cost, Delivery, Recipients, Segment, Tag and Forecast vs Actual sheets",
          "One-page PDF summary",
          "Large exports prepared in the background and kept for 7 days",
        ],
      },
    ],
    steps: [
      { title: "Launch the campaign", body: "The pre-launch analysis is fixed at start and compared with actuals later." },
      { title: "Map the reactions", body: "Mark the template's quick-reply buttons as positive or negative." },
      { title: "Read the results", body: "Check rates, cost and breakdowns in the analysis centre." },
      { title: "Act on them", body: "Select those who did not reply or replied positively, tag them or prepare a follow-up campaign." },
    ],
    plans: "The analysis centre, breakdowns and exports come with campaigns in every plan. Classifying free-text replies with AI uses your plan's monthly AI credits.",
    faq: [
      {
        q: "Can I see who blocked or reported my business?",
        a: "No. Meta does not say which person blocked or reported a business. Messora shows the aggregate signals: number and template quality, Meta's error codes and the opt-out rate.",
      },
      {
        q: "Which campaign does a reply count for?",
        a: "The campaign that reached the customer last, if the reply came in on the same number within the window you set (7 days by default). When a newer campaign reaches the customer, the earlier campaign's window ends.",
      },
      {
        q: "Does the cost figure match Meta's invoice?",
        a: "Messora calculates cost from Meta's rate card and shows it as \"System calculation\"; Meta's own daily cost data is fetched separately as well. The exact amount is on your Meta invoice.",
      },
      {
        q: "How often do the figures update?",
        a: "Live. Delivery, reads and replies update the screen as they arrive, without a refresh. The counters are also recalculated from the underlying rows every few minutes as a check.",
      },
      {
        q: "How do I send the report to my manager?",
        a: "Download the campaign's Excel report or one-page PDF summary and share it. The Excel report holds the summary, cost, delivery, recipients, segment, tag and forecast vs actual on separate sheets.",
      },
    ],
    related: ["campaigns", "ai", "api"],
  },
  {
    key: "api",
    slug: "api-and-webhooks",
    nav: "API and webhooks",
    icon: "code",
    seoTitle: "WhatsApp API and Webhook Integration",
    seoDescription: "Send WhatsApp messages from your own systems and receive events through signed webhooks. Integrate with scoped API keys and API docs for your own company.",
    eyebrow: "API and webhooks",
    h1: "WhatsApp API integration: send from your own systems, get events as they happen",
    lead: "Your order, booking or CRM system sends WhatsApp messages and manages contacts and campaigns through Messora's REST API. Signed webhooks tell your system about new messages, delivery status and opt-outs. The API base URL is https://app.messora.tech/api/v1, and your API key identifies your company. On Enterprise you use your own address.",
    summary: "Manage messages, contacts and campaigns over the REST API, and receive events in your system through signed webhooks.",
    highlights: ["Scoped API keys", "HMAC-SHA256 signed webhooks", "API docs per company"],
    blocks: [
      {
        title: "REST API",
        body: "Send messages and media, read and answer conversations, update contacts and their consent, read segments, create, start and pause campaigns and fetch their results. The campaign preflight and pre-launch analysis are in the API too. The reference is OpenAPI 3.1.",
      },
      {
        title: "Safe keys",
        body: "Give each API key only the permission it needs, for example sending messages only or reading contacts only. Each key is limited to 300 requests per minute.",
        points: [
          "Idempotency-Key stops a repeated request from running twice",
          "Consistent error format and pagination",
          "Creating and deleting keys is written to the audit log",
        ],
      },
      {
        title: "Webhook events",
        body: "Choose the events you want, and Messora sends a signed POST to your HTTPS address for each one. The event is recorded in the same database transaction as the change: a rolled-back change sends no event, and every committed change produces its event.",
        points: [
          "message.received, message.status",
          "conversation.assigned, conversation.closed, conversation.reopened",
          "campaign.completed, campaign.failed, campaign.cancelled",
          "contact.created, contact.updated, contact.opted_out, contact.opted_in",
        ],
      },
      {
        title: "Retries until delivered",
        body: "If your server does not answer, Messora retries 10 times over about 24 hours. The delivery log shows each attempt's status, HTTP code and duration, and you can redeliver with one click. Events and the delivery log are kept for 30 days.",
      },
      {
        title: "Signatures and network protection",
        body: "Every request carries an HMAC-SHA256 signature with your endpoint's secret and a timestamp, so you can verify it came from Messora unchanged. Webhook addresses cannot point to private or local network addresses; the check runs when you save and again on every connection.",
      },
      {
        title: "Interactive API documentation",
        body: "The API documentation is published in the panel; the base URL is https://app.messora.tech/api/v1 and your API key identifies the company. On Enterprise the docs and the API run at your own address. The interactive reference has request and response examples and code samples for every operation.",
      },
    ],
    steps: [
      { title: "Create a key", body: "Create an API key in the panel with only the permissions it needs." },
      { title: "Open the docs", body: "Try the request examples with your own key in the API documentation in the panel." },
      { title: "Add a webhook", body: "Register your HTTPS address, choose events and test your receiver with a test event." },
      { title: "Verify the signature", body: "Check every request's signature and timestamp, and drop duplicates by event ID." },
    ],
    plans: "Webhooks only on Starter; Growth 100k, Pro 1 million calls per month. Enterprise is set to your needs. The 300 requests per minute per key limit applies on every plan.",
    faq: [
      {
        q: "Do consent rules apply to messages sent through the API?",
        a: "Yes. A marketing template cannot be sent through the API to someone who opted out of marketing or stopped your business in WhatsApp; the request is refused with an error code naming the reason. No setting or parameter overrides this.",
      },
      {
        q: "What if I send the same request twice by mistake?",
        a: "Add an Idempotency-Key header. A second request with the same key is not processed again and returns the first result, so the customer does not get the message twice.",
      },
      {
        q: "Are events lost while my server is down?",
        a: "No. Messora retries failed deliveries 10 times over about 24 hours. If that runs out too, you can redeliver the event from the delivery log. Delivery is at least once, so drop duplicates by event ID.",
      },
      {
        q: "Why use Messora's API instead of Meta's Cloud API directly?",
        a: "Meta's API carries the message. Consent checks, contact and campaign management, the team inbox, cost calculation and signed events come ready in Messora, so you do not have to build the same rules yourself.",
      },
      {
        q: "Can I use the API on the Starter plan?",
        a: "On Starter you can receive events in your system through webhooks; API calls open with the Growth plan. Growth includes 100k calls a month, Pro 1 million.",
      },
    ],
    related: ["automations", "campaigns", "inbox"],
  },
];
