require 'rails_helper'

RSpec.describe Settings::ProfilesController, type: :controller do
  render_views

  before do
    @user = Fabricate(:user)
    sign_in @user, scope: :user
  end

  describe "GET #show" do
    before do
      get :show
    end

    it "returns http success" do
      expect(response).to have_http_status(:success)
    end

    it "sets Content-Security-Policy" do
      expect(response.headers["Content-Security-Policy"]).to eq "default-src 'none'; font-src 'self'; img-src 'self'; script-src 'self'; style-src 'self'"
    end
  end

  describe 'PUT #update' do
    it 'updates the user profile' do
      account = Fabricate(:account, user: @user, display_name: 'Old name')

      put :update, params: { account: { display_name: 'New name' } }
      expect(account.reload.display_name).to eq 'New name'
      expect(response).to redirect_to(settings_profile_path)
    end
  end
end
