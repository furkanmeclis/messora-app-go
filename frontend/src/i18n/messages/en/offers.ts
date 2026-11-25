const offers = {
    title: 'Offers',
    description:
        'Time-limited offers your campaigns promote. Limited-time offer templates count down to the end on WhatsApp.',
    nav: {
        label: 'Templates and offers',
        templates: 'Templates',
        offers: 'Offers',
    },
    new: 'New offer',
    edit: 'Edit offer',
    create: 'Create offer',
    dialogHint:
        'Campaigns linked to the offer take its end, coupon and image; they stop when it ends.',
    status: 'Status',
    statuses: {
        draft: 'Draft',
        scheduled: 'Scheduled',
        active: 'Active',
        expired: 'Ended',
    },
    fields: {
        title: 'Title',
        discount: 'Discount',
        discountPlaceholder: '25% off',
        coupon: 'Coupon code',
        starts: 'Starts',
        ends: 'Ends',
        product: 'Product or service',
        description: 'Description',
        image: 'Image (https)',
        cta: 'Offer page',
        template: 'Template',
    },
    noTemplate: 'No template',
    ltoTag: 'countdown',
    publish: 'Publish (active between start and end)',
    preview: 'Preview',
    ltoNote:
        'Limited-time offer template: WhatsApp shows the countdown to the end and the coupon. WhatsApp Web and desktop show that the offer is not supported there.',
    fallbackNote:
        "Only limited-time offer templates show WhatsApp's countdown. With a normal marketing template, the offer's image fills an empty image header and the placeholders {{offer_title}}, {{offer_discount}}, {{offer_coupon}}, {{offer_ends}} and {{offer_url}} carry the rest.",
    empty: 'No offers yet',
    emptyHint:
        'Create an offer, link it to a campaign and it stops the campaign when it ends.',
    editLabel: 'Edit {title}',
    endLabel: 'End {title} now',
    deleteLabel: 'Delete {title}',
    deleteTitle: 'Delete {title}?',
    deleteBody:
        'Campaigns that used it keep their own values; nothing is sent again.',
    created: 'Offer created.',
    saved: 'Offer saved.',
    ended: 'Offer ended; campaigns using it are paused.',
    deleted: 'Offer deleted.',
    campaign: {
        label: 'Offer',
        none: 'No offer',
        hint: 'Linked offers fill the countdown end, coupon and image, and pause the campaign when they end.',
        ended: 'This offer has ended; the campaign cannot start with it.',
    },
};
export default offers;
