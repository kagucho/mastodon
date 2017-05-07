require 'rails_helper'

RSpec.describe Api::V1::StatusesController, type: :controller do
  render_views

  let(:user)  { Fabricate(:user, account: Fabricate(:account, username: 'alice')) }
  let(:app)   { Fabricate(:application, name: 'Test app', website: 'http://testapp.com') }
  let(:token) { double acceptable?: true, resource_owner_id: user.id, application: app }

  context 'with an oauth token' do
    before do
      allow(controller).to receive(:doorkeeper_token) { token }
    end

    describe 'GET #show' do
      let(:status) { Fabricate(:status, account: user.account) }

      it 'returns http success' do
        get :show, params: { id: status.id }
        expect(response).to have_http_status(:success)
      end
    end

    describe 'GET #context' do
      let(:status) { Fabricate(:status, account: user.account) }

      before do
        Fabricate(:status, account: user.account, thread: status)
      end

      it 'returns http success' do
        get :context, params: { id: status.id }
        expect(response).to have_http_status(:success)
      end
    end

    describe 'POST #create' do
      before do
        post :create, params: { status: 'Hello world' }
      end

      it 'returns http success' do
        expect(response).to have_http_status(:success)
      end
    end

    describe 'DELETE #destroy' do
      let(:status) { Fabricate(:status, account: user.account) }

      before do
        post :destroy, params: { id: status.id }
      end

      it 'returns http success' do
        expect(response).to have_http_status(:success)
      end

      it 'removes the status' do
        expect(Status.find_by(id: status.id)).to be nil
      end
    end
  end

  context 'without an oauth token' do
    before do
      allow(controller).to receive(:doorkeeper_token) { nil }
    end

    context 'with a private status' do
      let(:status) { Fabricate(:status, account: user.account, visibility: :private) }

      describe 'GET #show' do
        it 'returns http unautharized' do
          get :show, params: { id: status.id }
          expect(response).to have_http_status(:missing)
        end
      end

      describe 'GET #context' do
        before do
          Fabricate(:status, account: user.account, thread: status)
        end

        it 'returns http unautharized' do
          get :context, params: { id: status.id }
          expect(response).to have_http_status(:missing)
        end
      end

      describe 'GET #card' do
        it 'returns http unautharized' do
          get :card, params: { id: status.id }
          expect(response).to have_http_status(:missing)
        end
      end

      describe 'GET #card_html' do
        it 'returns http unautharized' do
          get :card_html, params: { id: status.id }
          expect(response).to have_http_status(:missing)
        end
      end
    end

    context 'with a public status' do
      let(:status) { Fabricate(:status, account: user.account, visibility: :public) }

      describe 'GET #show' do
        it 'returns http success' do
          get :show, params: { id: status.id }
          expect(response).to have_http_status(:success)
        end
      end

      describe 'GET #context' do
        before do
          Fabricate(:status, account: user.account, thread: status)
        end

        it 'returns http success' do
          get :context, params: { id: status.id }
          expect(response).to have_http_status(:success)
        end
      end

      describe 'GET #card' do
        it 'returns http success' do
          get :card, params: { id: status.id }
          expect(response).to have_http_status(:success)
        end
      end

      describe 'GET #card_html' do
        context 'without card' do
          it 'returns http missing' do
            get :card_html, params: { id: Fabricate(:status, account: user.account).id }
            expect(response).to have_http_status(:missing)
          end
        end

        context 'with card' do
          before do
            get :card_html, params: {
              id: Fabricate(:status,
                account: user.account,
                preview_card: Fabricate(:preview_card)).id
            }
          end

          it 'returns http success' do
            expect(response).to have_http_status(:success)
          end

          it 'sets Content-Security-Policy' do
            expect(response.headers['Content-Security-Policy']).to eq "connect-src 'none'; script-src 'none'"
          end

          it 'sets X-Frame-Options' do
            expect(response.headers['X-Frame-Options']).to eq 'ALLOWALL'
          end
        end
      end
    end
  end
end
