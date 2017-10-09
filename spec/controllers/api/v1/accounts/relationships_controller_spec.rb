require 'rails_helper'

describe Api::V1::Accounts::RelationshipsController do
  render_views

  let(:user)  { Fabricate(:user, account: Fabricate(:account, username: 'alice')) }
  let(:token) { Fabricate(:accessible_access_token, resource_owner_id: user.id, scopes: 'read') }

  before do
    allow(controller).to receive(:doorkeeper_token) { token }
  end

  describe 'GET #index' do
    let(:simon) { Fabricate(:user, email: 'simon@example.com', account: Fabricate(:account, username: 'simon')).account }
    let(:lewis) { Fabricate(:user, email: 'lewis@example.com', account: Fabricate(:account, username: 'lewis')).account }

    before do
      user.account.follow!(simon)
      lewis.follow!(user.account)
    end

    context 'provided only one ID' do
      before do
        get :index, params: { id: simon.id }
      end

      it 'returns http success' do
        expect(response).to have_http_status(:success)
      end

      it 'returns JSON with correct data' do
        json = body_as_json

        expect(json).to be_a Enumerable
        expect(json.first[:following]).to be true
        expect(json.first[:followed_by]).to be false
      end
    end

    context 'provided multiple IDs' do
      before do
        get :index, params: { id: [simon.id, lewis.id] }
      end

      it 'returns http success' do
        expect(response).to have_http_status(:success)
      end

      it 'returns JSON with correct data' do
        json = body_as_json

        simon_json = json.find { |element| element[:id] == simon.id.to_s }
        expect(simon_json[:following]).to be true
        expect(simon_json[:followed_by]).to be false
        expect(simon_json[:muting]).to be false
        expect(simon_json[:requested]).to be false
        expect(simon_json[:domain_blocking]).to be false

        lewis_json = json.find { |element| element[:id] == lewis.id.to_s }
        expect(lewis_json[:following]).to be false
        expect(lewis_json[:followed_by]).to be true
        expect(lewis_json[:muting]).to be false
        expect(lewis_json[:requested]).to be false
        expect(lewis_json[:domain_blocking]).to be false
      end
    end
  end
end
