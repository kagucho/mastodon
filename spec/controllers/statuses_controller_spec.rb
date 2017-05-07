# frozen_string_literal: true

require 'rails_helper'

describe StatusesController do
  render_views

  describe '#show' do
    context 'account is suspended' do
      it 'returns gone' do
        account = Fabricate(:account, suspended: true)
        status = Fabricate(:status, account: account)

        get :show, params: { account_username: account.username, id: status.id }

        expect(response).to have_http_status(410)
      end
    end

    context 'status is not permitted' do
      it 'raises ActiveRecord::RecordNotFound' do
        user = Fabricate(:user)
        status = Fabricate(:status)
        status.account.block!(user.account)

        sign_in(user)
        get :show, params: { account_username: status.account.username, id: status.id }

        expect(response).to have_http_status(404)
      end
    end

    context 'account is not suspended and status is permitted' do
      it 'assigns @account' do
        status = Fabricate(:status)
        get :show, params: { account_username: status.account.username, id: status.id }
        expect(assigns(:account)).to eq status.account
      end

      it 'assigns @status' do
        status = Fabricate(:status)
        get :show, params: { account_username: status.account.username, id: status.id }
        expect(assigns(:status)).to eq status
      end

      it 'assigns @stream_entry' do
        status = Fabricate(:status)
        get :show, params: { account_username: status.account.username, id: status.id }
        expect(assigns(:stream_entry)).to eq status.stream_entry
      end

      it 'assigns @type' do
        status = Fabricate(:status)
        get :show, params: { account_username: status.account.username, id: status.id }
        expect(assigns(:type)).to eq 'status'
      end

      it 'assigns @ancestors for ancestors of the status if it is a reply' do
        ancestor = Fabricate(:status)
        status = Fabricate(:status, in_reply_to_id: ancestor.id)

        get :show, params: { account_username: status.account.username, id: status.id }

        expect(assigns(:ancestors)).to eq [ancestor]
      end

      it 'assigns @ancestors for [] if it is not a reply' do
        status = Fabricate(:status)
        get :show, params: { account_username: status.account.username, id: status.id }
        expect(assigns(:ancestors)).to eq []
      end

      it 'returns a success' do
        status = Fabricate(:status)
        get :show, params: { account_username: status.account.username, id: status.id }
        expect(response).to have_http_status(:success)
      end

      it 'sets Content-Security-Policy' do
        status = Fabricate(:status)
        get :show, params: { account_username: status.account.username, id: status.id }
        expect(response.headers['Content-Security-Policy']).to eq "default-src 'none'; font-src 'self'; img-src 'self'; media-src 'self'; script-src 'self'; style-src 'self' 'sha256-qRrgHHCDuxyr4+DLfz1F4BlFayvcGq1hI47y1mNEEQo=' 'sha256-CDezrqMbKJf+dAHyv8FcpR+fJE6UjOO+TRZSTzRLwgo=' 'sha256-Ki4+BbA7mzce+JTtl+Tkj6UE1essH0OVxVREUC/e0ZE=' 'sha256-z0vtjdbuC6Uv1IWeBkO1oUlQY53+gsWMOJRtt4G3wQY=' 'sha256-S7898Hb+PHYyBv4itdTHM50tQsn3LF2RDisrLLd4BLE='"
      end

      it 'renders stream_entries/show' do
        status = Fabricate(:status)
        get :show, params: { account_username: status.account.username, id: status.id }
        expect(response).to render_template 'stream_entries/show'
      end
    end
  end
end
