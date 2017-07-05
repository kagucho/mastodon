# frozen_string_literal: true

require 'rails_helper'

describe Api::ActivityIntentsController, type: :controller do
  describe 'follow' do
    it 'assigns @account' do
      account = Fabricate(:account)
      get :follow, params: { id: account.username }
      p response.body # TODO
    end
  end
end
