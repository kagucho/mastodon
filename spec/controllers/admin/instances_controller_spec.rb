require 'rails_helper'

RSpec.describe Admin::InstancesController, type: :controller do
  render_views

  before do
    sign_in Fabricate(:user, admin: true), scope: :user
  end

  describe 'GET #index' do
    around do |example|
      default_per_page = Account.default_per_page
      Account.paginates_per 1
      example.run
      Account.paginates_per default_per_page
    end

    it 'renders instances' do
      Fabricate(:account, domain: 'popular')
      Fabricate(:account, domain: 'popular')
      Fabricate(:account, domain: 'less.popular')

      get :index, params: { page: 2 }

      instances = assigns(:instances).to_a
      expect(instances.size).to eq 1
      expect(instances[0].domain).to eq 'less.popular'
      expect(response.headers['Content-Security-Policy']).to eq "default-src 'none'; font-src 'self'; img-src 'self'; script-src 'self'; style-src 'self'"
    end
  end
end
