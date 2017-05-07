require 'rails_helper'

RSpec.describe AccountsController, type: :controller do
  render_views

  let(:alice)  { Fabricate(:account, username: 'alice') }

  describe 'GET #show' do
    let!(:status1) { Status.create!(account: alice, text: 'Hello world') }
    let!(:status2) { Status.create!(account: alice, text: 'Boop', thread: status1) }
    let!(:status3) { Status.create!(account: alice, text: 'Picture!') }
    let!(:status4) { Status.create!(account: alice, text: 'Mentioning @alice') }

    before do
      status3.media_attachments.create!(account: alice, file: fixture_file_upload('files/attachment.jpg', 'image/jpeg'))
    end

    context 'atom' do
      before do
        get :show, params: { username: alice.username, max_id: status4.stream_entry.id, since_id: status1.stream_entry.id }, format: 'atom'
      end

      it 'assigns @account' do
        expect(assigns(:account)).to eq alice
      end

      it 'assigns @entries' do
        entries = assigns(:entries).to_a
        expect(entries.size).to eq 2
        expect(entries[0].status).to eq status3
        expect(entries[1].status).to eq status2
      end

      it 'returns http success with Atom' do
        expect(response).to have_http_status(:success)
      end
    end

    context 'activitystreams2' do
      before do
        get :show, params: { username: alice.username }, format: 'activitystreams2'
      end

      it 'assigns @account' do
        expect(assigns(:account)).to eq alice
      end

      it 'returns http success with Activity Streams 2.0' do
        expect(response).to have_http_status(:success)
      end
    end

    context 'html' do
      before do
        get :show, params: { username: alice.username, max_id: status4.id, since_id: status1.id }
      end

      it 'assigns @account' do
        expect(assigns(:account)).to eq alice
      end

      it 'assigns @statuses' do
        statuses = assigns(:statuses).to_a
        expect(statuses.size).to eq 2
        expect(statuses[0]).to eq status3
        expect(statuses[1]).to eq status2
      end

      it 'returns http success' do
        expect(response).to have_http_status(:success)
      end

      it 'sets Content-Security-Policy' do
        expect(response.headers['Content-Security-Policy']).to eq "default-src 'none'; font-src 'self'; img-src 'self'; script-src 'self'; style-src 'self' 'sha256-47DEQpj8HBSa+/TImW+5JCeuQeRkm5NMpJWZG3hSuFU=' 'sha256-qRrgHHCDuxyr4+DLfz1F4BlFayvcGq1hI47y1mNEEQo=' 'sha256-CDezrqMbKJf+dAHyv8FcpR+fJE6UjOO+TRZSTzRLwgo=' 'sha256-Ki4+BbA7mzce+JTtl+Tkj6UE1essH0OVxVREUC/e0ZE=' 'sha256-z0vtjdbuC6Uv1IWeBkO1oUlQY53+gsWMOJRtt4G3wQY=' 'sha256-S7898Hb+PHYyBv4itdTHM50tQsn3LF2RDisrLLd4BLE='"
      end
    end
  end
end
