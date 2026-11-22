<?php

// Temporary Phase 0 verification probes for Messora. Each test asserts the
// CURRENT (suspected buggy) behaviour, so a passing test confirms the finding.

use App\Models\AutomationFlow;
use App\Models\Campaign;
use App\Models\ContactGroup;
use App\Models\Tenant;
use App\Models\User;
use App\Models\WhatsappAccount;
use App\Models\WhatsappChat;
use App\Models\CampaignRecipient;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Illuminate\Support\Facades\Http;

uses(RefreshDatabase::class);

beforeEach(function () {
    config(['whatsapp.app_secret' => 'test-secret']);
    $this->tenant = Tenant::create(['name' => 'T', 'email' => 't@t.com', 'status' => 'active']);
    $this->user = User::create(['name' => 'U', 'email' => 'u@t.com', 'password' => bcrypt('x'), 'role' => 'client', 'tenant_id' => $this->tenant->id]);
    $this->account = WhatsappAccount::create([
        'tenant_id' => $this->tenant->id, 'waba_id' => 'W1', 'phone_number_id' => 'P1',
        'phone_number' => '+100', 'status' => 'active', 'access_token' => 'TOKEN-PLAINTEXT', 'app_secret' => 'APPSECRET-PLAINTEXT',
    ]);
});

function signedWebhook($test, array $payload, string $token)
{
    $body = json_encode($payload);
    $sig = 'sha256='.hash_hmac('sha256', $body, 'APPSECRET-PLAINTEXT');

    return $test->call('POST', "/webhook/whatsapp/{$token}", [], [], [], [
        'HTTP_X_HUB_SIGNATURE_256' => $sig, 'CONTENT_TYPE' => 'application/json',
    ], $body);
}

function inbound(string $id, string $text): array
{
    return ['entry' => [['changes' => [['field' => 'messages', 'value' => [
        'metadata' => ['phone_number_id' => 'P1'],
        'contacts' => [['wa_id' => '905551112233', 'profile' => ['name' => 'Ali']]],
        'messages' => [['from' => '905551112233', 'id' => $id, 'timestamp' => (string) time(), 'type' => 'text', 'text' => ['body' => $text]]],
    ]]]]]];
}

test('F1 resume: starting a paused campaign leaves it paused and sends nothing', function () {
    Http::fake();
    $c = Campaign::create(['tenant_id' => $this->tenant->id, 'whatsapp_account_id' => $this->account->id,
        'message_type' => 'direct', 'direct_message_body' => 'hi', 'name' => 'c', 'status' => 'paused', 'total_recipients' => 1]);
    $c->recipients()->create(['phone_number' => '+905551112233']);

    $this->actingAs($this->user)->post("/dashboard/campaigns/{$c->id}/start");

    expect($c->fresh()->status)->toBe('paused');
    expect(CampaignRecipient::first()->status)->toBe('pending');
    Http::assertNothingSent();
});

test('F2 automation send_message throws a PHP Error and the webhook returns 500', function () {
    Http::fake();
    AutomationFlow::create(['tenant_id' => $this->tenant->id, 'name' => 'f', 'trigger_keyword' => '*',
        'trigger_match_type' => 'contains', 'actions' => [['type' => 'send_message', 'text' => 'auto']], 'is_active' => true]);

    $this->withoutExceptionHandling();
    expect(fn () => signedWebhook($this, inbound('wamid.1', 'hello'), $this->tenant->webhook_token))
        ->toThrow(ArgumentCountError::class);
});

test('F3 duplicate webhook delivery re-runs automations (http_request fires twice)', function () {
    Http::fake();
    AutomationFlow::create(['tenant_id' => $this->tenant->id, 'name' => 'f', 'trigger_keyword' => '*',
        'trigger_match_type' => 'contains', 'actions' => [['type' => 'http_request', 'method' => 'POST', 'url' => 'https://example.test/hook']], 'is_active' => true]);

    signedWebhook($this, inbound('wamid.dup', 'hello'), $this->tenant->webhook_token)->assertOk();
    signedWebhook($this, inbound('wamid.dup', 'hello'), $this->tenant->webhook_token)->assertOk();

    expect(WhatsappChat::first()->messages()->count())->toBe(1);
    Http::assertSentCount(2);
});

test('F4 webhook verification ignores hub.verify_token', function () {
    $this->get("/webhook/whatsapp/{$this->tenant->webhook_token}?hub_mode=subscribe&hub_verify_token=WRONG&hub_challenge=42")
        ->assertOk()->assertSee('42');
});

test('F5 out-of-order status webhook downgrades read back to delivered', function () {
    $c = Campaign::create(['tenant_id' => $this->tenant->id, 'whatsapp_account_id' => $this->account->id,
        'message_type' => 'direct', 'name' => 'c', 'status' => 'completed', 'total_recipients' => 1]);
    $c->recipients()->create(['phone_number' => '+1', 'status' => 'sent', 'whatsapp_message_id' => 'wamid.s']);
    $st = fn ($s) => ['entry' => [['changes' => [['field' => 'messages', 'value' => [
        'metadata' => ['phone_number_id' => 'P1'], 'statuses' => [['id' => 'wamid.s', 'status' => $s, 'timestamp' => (string) time()]]]]]]]];

    signedWebhook($this, $st('read'), $this->tenant->webhook_token);
    signedWebhook($this, $st('delivered'), $this->tenant->webhook_token);

    expect(CampaignRecipient::first()->status)->toBe('delivered');
});

test('F6 whatsapp accounts page ships access_token and app_secret to the browser; DB stores them in plaintext', function () {
    $this->actingAs($this->user)->get('/dashboard/whatsapp-accounts')
        ->assertInertia(fn ($p) => $p->where('accounts.0.access_token', 'TOKEN-PLAINTEXT')
            ->where('accounts.0.app_secret', 'APPSECRET-PLAINTEXT')->etc());
    expect(\DB::table('whatsapp_accounts')->value('access_token'))->toBe('TOKEN-PLAINTEXT');
});

test('F7 contact can be attached to another tenant\'s group (IDOR via group_ids)', function () {
    $other = Tenant::create(['name' => 'O', 'email' => 'o@t.com', 'status' => 'active']);
    $foreign = ContactGroup::create(['tenant_id' => $other->id, 'name' => 'foreign']);

    $r = $this->actingAs($this->user)->post('/dashboard/contacts', ['name' => 'x', 'phone' => '+905550000000', 'group_ids' => [$foreign->id]]);
    fwrite(STDERR, 'F7 status='.$r->status().' errors='.json_encode(session('errors')?->all())."\n");
    $r2 = $this->actingAs($this->user)->post('/dashboard/contacts', ['name' => 'y', 'phone' => '+905550000001', 'email' => null, 'notes' => null, 'group_ids' => [$foreign->id]]);
    fwrite(STDERR, 'F7b status='.$r2->status()."\n");

    expect($foreign->contacts()->count())->toBe(1);
});

test('F8 contact-groups index renders a page component that does not exist', function () {
    expect(file_exists(resource_path('js/pages/client/contact-groups/index.tsx')))->toBeFalse();
});

test('F9 direct (free-form) campaign is sent to recipients without any 24h session check', function () {
    Http::fake(['*' => Http::response(['messages' => [['id' => 'wamid.x']]])]);
    $c = Campaign::create(['tenant_id' => $this->tenant->id, 'whatsapp_account_id' => $this->account->id,
        'message_type' => 'direct', 'direct_message_body' => 'promo', 'name' => 'c', 'status' => 'draft', 'total_recipients' => 1]);
    $c->recipients()->create(['phone_number' => '+905559999999']);

    $this->actingAs($this->user)->post("/dashboard/campaigns/{$c->id}/start");

    Http::assertSent(fn ($r) => ($r['type'] ?? null) === 'text');
});

test('F10 campaign send exception marks recipient failed with no retry (timeout => failed)', function () {
    Http::fake(['*' => fn () => throw new \Illuminate\Http\Client\ConnectionException('timeout')]);
    $c = Campaign::create(['tenant_id' => $this->tenant->id, 'whatsapp_account_id' => $this->account->id,
        'message_type' => 'direct', 'direct_message_body' => 'x', 'name' => 'c', 'status' => 'draft', 'total_recipients' => 1]);
    $c->recipients()->create(['phone_number' => '+905559999999']);

    $this->actingAs($this->user)->post("/dashboard/campaigns/{$c->id}/start");

    expect(CampaignRecipient::first()->status)->toBe('failed');
    Http::assertSentCount(0); // exception thrown before recording; no second attempt either
});
