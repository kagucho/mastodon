require 'rails_helper'

RSpec.describe ContentSecurityPolicy do
  describe 'self.digest' do
    context 'falsy' do
      it "returns 'sha256-47DEQpj8HBSa+/TImW+5JCeuQeRkm5NMpJWZG3hSuFU='" do
        expect(described_class.digest(nil)).to eq "'sha256-47DEQpj8HBSa+/TImW+5JCeuQeRkm5NMpJWZG3hSuFU='"
      end
    end

    context 'string' do
      it "returns 'sha256-47DEQpj8HBSa+/TImW+5JCeuQeRkm5NMpJWZG3hSuFU='" do
        expect(described_class.digest('')).to eq "'sha256-47DEQpj8HBSa+/TImW+5JCeuQeRkm5NMpJWZG3hSuFU='"
      end
    end
  end
end
